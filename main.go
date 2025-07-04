package main

import (
	"context"
	"log"
	"os"

	// Generador de IDs únicos

	"github.com/gin-gonic/gin" // Framework web
	"github.com/gjae/go-recipes-api/handlers"
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

var recipeHandler *handlers.RecipeHandler

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

	recipeHandler = handlers.NewRecipeHandler(ctx, collection)
}

// main configura el servidor HTTP y las rutas
func main() {
	// Crear enrutador Gin con middleware por defecto
	router := gin.Default()

	// Configurar rutas para el CRUD de recetas
	router.POST("/recipes", recipeHandler.NewRecipeHandler)          // Crear
	router.GET("/recipes", recipeHandler.ListRecipeHandler)          // Listar
	router.PUT("/recipes/:id", recipeHandler.UpdateRecipeHandler)    // Actualizar
	router.DELETE("/recipes/:id", recipeHandler.DeleteRecipeHandler) // Eliminar
	router.GET("/recipes/search", recipeHandler.SearchRecipeHandler) // Buscar por tag
	router.GET("/recipes/:id", recipeHandler.SearchRecipeByID)       // Obtener por ID

	// Ruta de prueba
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "API de Recetas funcionando",
		})
	})

	// Iniciar servidor en el puerto por defecto (8080)
	log.Println("Servidor iniciado en http://localhost:8080")
	router.Run()
}
