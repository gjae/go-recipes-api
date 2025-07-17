package main

import (
	"context"
	"log"
	"os"

	// Generador de IDs únicos

	"github.com/gin-contrib/sessions"
	redisStore "github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin" // Framework web
	"github.com/gjae/go-recipes-api/handlers"
	"github.com/go-redis/redis"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Variables globales
var (
	ctx    context.Context // Contexto para operaciones con MongoDB
	err    error           // Variable para manejo de errores
	client *mongo.Client   // Cliente de MongoDB
)

var recipeHandler *handlers.RecipeHandler // Handler para las recetas
var authHandler *handlers.AuthHandler

// init se ejecuta al iniciar la aplicación
func init() {

	// Configurar contexto para MongoDB
	ctx = context.Background()

	// Conectar a MongoDB usando URI de variable de entorno
	client, err = mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		log.Fatal("Error al conectar a MongoDB:", err)
	}

	// Verificar conexión con ping al servidor
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		log.Fatal("Error al hacer ping a MongoDB:", err)
	}
	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")

	// Crear archivo marcador para evitar inserción duplicada
	if _, err := os.Create(".data_loaded"); err != nil {
		log.Println("Error al crear archivo .data_loaded:", err)
	}

	// Inicializar el handler de recetas con la conexión a la base de datos y Redis
	recipeHandler = handlers.NewRecipeHandler(ctx, collection, redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}))
	// Inicializar el handler de autenticación con la conexión a la base de datos
	authHandler = handlers.NewAuthHandler(client.Database("MONGO_DATABASE").Collection("users"), ctx)
	recipeHandler.MakePing()
}

// main configura el servidor HTTP y define las rutas de la API
func main() {

	// Crear enrutador Gin con middleware por defecto
	store, _ := redisStore.NewStore(10, "tcp", "localhost:6379", "", "")
	router := gin.Default()

	router.RunTLS(":443", "certs/localhost.crt", "certs/localhost.key")
	// Configurar el almacenamiento de sesiones con Redis
	router.Use(sessions.Sessions("recipes_api", store))
	// Crear un grupo de rutas autorizadas

	authorized := router.Group("/")

	//authorized.Use(authHandler.AuthMiddleware())
	authorized.Use(authHandler.AuthMiddleware())
	{
		authorized.POST("/recipes", recipeHandler.NewRecipeHandler)          // Crear
		authorized.GET("/recipes", recipeHandler.ListRecipeHandler)          // Listar
		authorized.PUT("/recipes/:id", recipeHandler.UpdateRecipeHandler)    // Actualizar
		authorized.DELETE("/recipes/:id", recipeHandler.DeleteRecipeHandler) // Eliminar
	}
	// Configurar rutas para la búsqueda de recetas
	router.GET("/recipes/search", recipeHandler.SearchRecipeHandler) // Buscar por tag
	router.GET("/recipes/:id", recipeHandler.SearchRecipeByID)       // Obtener por ID

	// Configurar rutas para la autenticación
	router.POST("/signin", authHandler.SignInHandler)
	router.POST("/refresh", authHandler.RefreshTokenHandler)

	// Definir una ruta de prueba en la raíz
	router.GET("/", func(c *gin.Context) {
		// Responder con un mensaje JSON indicando que la API está funcionando

		c.JSON(200, gin.H{
			"message": "API de Recetas funcionando",
		})
	})

	// Iniciar servidor en el puerto por defecto (8080)
	router.RunTLS(":8005", "certs/localhost.crt", "certs/localhost.key")
}
