package handlers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gjae/go-recipes-api/models"
	"github.com/rs/xid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"github.com/auth0-community/go-auth0"
	jose "gopkg.in/square/go-jose.v2"
)

// Claims define la estructura de las reclamaciones (claims) del JWT.
// Incluye el nombre de usuario y las reclamaciones estándar de JWT.
type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

// JWTOutput define la estructura de la salida JWT, incluyendo el token y su fecha de expiración.
type JWTOutput struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

// AuthHandler es una estructura que contiene las dependencias para manejar la autenticación.
type AuthHandler struct {
	collection *mongo.Collection // Colección de MongoDB para usuarios.
	ctx        context.Context   // Contexto para las operaciones de base de datos.
}

// NewAuthHandler crea una nueva instancia de AuthHandler.
// Recibe una colección de MongoDB y un contexto como dependencias.
func NewAuthHandler(collection *mongo.Collection, ctx context.Context) *AuthHandler {
	return &AuthHandler{
		collection: collection,
		ctx:        ctx,
	}
}

// SessionAuthMiddleware es un middleware para la autenticación basada en sesiones.
// Verifica la presencia y validez de un token JWT en el encabezado de autorización.
func (handler *AuthHandler) SessionAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Obtiene el token del encabezado de autorización.
		tokenValue := c.GetHeader("Authorization")
		claims := &Claims{}
		// Parsea y valida el token JWT.
		tkn, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (interface{}, error) {
			// Utiliza la clave secreta para verificar la firma del token.
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
		if tkn == nil || !tkn.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
		} // Continúa con el siguiente handler si la autenticación es exitosa.
		c.Next()
	}
}

// AuthMiddleware es un middleware para la autenticación basada en tokens JWT de Auth0.
// Valida el token JWT utilizando la configuración de Auth0.
func (handler *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Define el dominio de Auth0.
		var auth0Domain = fmt.Sprintf("https://%s/", os.Getenv("AUTH0_DOMAIN"))
		fmt.Println("Dominio: ", auth0Domain)
		// Crea un cliente JWK para obtener las claves públicas de Auth0.
		client := auth0.NewJWKClient(auth0.JWKClientOptions{URI: fmt.Sprintf("%s.well-known/jwks.json", auth0Domain)}, nil)

		// Configura el validador de Auth0.
		configuration := auth0.NewConfiguration(client, []string{os.Getenv("AUTH0_API_IDENTIFIER")}, auth0Domain, jose.RS256)
		validator := auth0.NewValidator(configuration, nil)
		// Valida la solicitud utilizando el validador de Auth0.
		_, err := validator.ValidateRequest(c.Request)
		// Si la validación falla, responde con un error de no autorizado.
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid token", "auth0Error: ": err.Error()})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CookieAuthMiddleware es un middleware para la autenticación basada en cookies.
// Verifica la presencia de un token de sesión en las cookies.
func (handler *AuthHandler) CookieAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Obtiene la sesión actual.
		session := sessions.Default(c)
		// Obtiene el token de sesión.
		sessionToken := session.Get("token")
		fmt.Println("Handler middleware: ", sessionToken)
		if sessionToken == nil {
			c.JSON(http.StatusForbidden, gin.H{"message": "User logged"})
			c.Abort()
		}
		c.Next()
	}
}

// JWTSignInHandler maneja el inicio de sesión (sign-in) basado en JWT.
// Recibe las credenciales del usuario, las valida y genera un JWT.
func (handler *AuthHandler) JWTSignInHandler(c *gin.Context) {

	var user models.User

	// Bindea el JSON del cuerpo de la solicitud a la estructura User.
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	// Hashea la contraseña del usuario usando SHA256.
	h := sha256.New()
	h.Write([]byte(user.Password))
	hashedPassword := fmt.Sprintf("%x", h.Sum(nil)) // Convert to hex string

	// Busca un usuario en la base de datos con el nombre de usuario y la contraseña hasheada.
	var userAuth models.User
	err := handler.collection.FindOne(handler.ctx, bson.M{
		"username": user.Username,
		"password": hashedPassword,
	}).Decode(&userAuth)

	fmt.Println("Buscando ", user.Username, " Password ", hashedPassword)

	// Si no se encuentra el usuario, responde con un error.
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Invalid username or password: %v", err.Error())})
		return
	}

	expirationTime := time.Now().Add(10 * time.Minute)
	claims := &Claims{
		Username: user.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Crea la salida JWT con el token y la fecha de expiración.
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}

	c.JSON(http.StatusOK, jwtOutput)
}

// SignInHandler maneja el inicio de sesión (sign-in) basado en sesiones.
// Recibe las credenciales del usuario, las valida y establece una sesión.
func (handler *AuthHandler) SignInHandler(c *gin.Context) {

	var user models.User

	// Bindea el JSON del cuerpo de la solicitud a la estructura User.
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	h := sha256.New()
	h.Write([]byte(user.Password))
	hashedPassword := fmt.Sprintf("%x", h.Sum(nil)) // Convert to hex string

	// Busca un usuario en la base de datos con el nombre de usuario proporcionado.
	var userAuth models.User
	err := handler.collection.FindOne(handler.ctx, bson.M{
		"username": user.Username,
	}).Decode(&userAuth)

	fmt.Println("Buscando ", user.Username, " Password ", hashedPassword, userAuth)

	// Si no se encuentra el usuario o la contraseña no coincide, responde con un error.
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Invalid username or password: %v", err.Error())})
		return
	}

	// Genera un token de sesión único.
	sessionToken := xid.New().String()
	// Obtiene la sesión actual.
	session := sessions.Default(c)

	fmt.Println("Session token", sessionToken)
	// Establece el nombre de usuario y el token en la sesión.
	session.Set("username", user.Username)
	session.Set("token", sessionToken)
	// Guarda la sesión.
	session.Save()
	c.JSON(http.StatusOK, gin.H{"message": "User signied"})
}

// RefreshTokenHandler maneja la actualización (refresh) de un token JWT.
// Recibe un token JWT, verifica su validez y genera un nuevo token con una nueva fecha de expiración.
func (handler *AuthHandler) RefreshTokenHandler(c *gin.Context) {
	// Obtiene el token del encabezado de autorización.
	tokenValue := c.GetHeader("Authorization")
	claims := &Claims{}

	// Parsea y valida el token JWT.
	tkn, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (interface{}, error) {
		// Utiliza la clave secreta para verificar la firma del token.
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} // Si el token no es válido, responde con un error.


	if tkn == nil || !tkn.Valid {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token"})
		return
	}

	if time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) > 30*time.Second {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is not expired yet"})
		return
	}

	// Establece una nueva fecha de expiración para el token.
	expirationTime := time.Now().Add(5 * time.Minute)
	claims.ExpiresAt = expirationTime.Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(os.Getenv("JWT_SECRET"))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Crea la salida JWT con el nuevo token y la fecha de expiración.
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}

	c.JSON(http.StatusOK, jwtOutput)
}
