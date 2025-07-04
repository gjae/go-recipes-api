package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	// Generador de IDs únicos

	"github.com/gin-gonic/gin" // Framework web
	"github.com/gjae/go-recipes-api/handlers"
	"github.com/gjae/go-recipes-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

// NewRecipeHandler maneja la creación de nuevas recetas
func NewRecipeHandler(c *gin.Context) {
	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	var recipe models.Recipe

	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	recipe.ID = primitive.NewObjectID().Hex()
	recipe.PublishedAt = time.Now()

	_, err := collection.InsertOne(ctx, recipe)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, recipe)
}

// UpdateRecipeHandler actualiza una receta existente
func UpdateRecipeHandler(c *gin.Context) {
	// Obtener ID de los parámetros de la URL
	id := c.Param("id")
	var recipe models.Recipe

	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	objectId, _ := primitive.ObjectIDFromHex(id)

	// Bindear JSON del request
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Datos de receta inválidos: " + err.Error(),
		})
		return
	}

	_, err := collection.UpdateOne(ctx, bson.M{
		"_id": objectId,
	}, bson.D{{"$set", bson.D{
		{"name", recipe.Name},
		{"tags", recipe.Tags},
		{"ingredients", recipe.Ingredients},
		{"instructions", recipe.Instructions},
	}}})

	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "models.Recipe has been updated"})
}

// DeleteRecipeHandler elimina una receta (Nota: Corregir nombre de función - typo en "DeleteRecippeHandler")
func DeleteRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")

	objectId, _ := primitive.ObjectIDFromHex(id)
	_, err := collection.DeleteOne(ctx, bson.M{"_id": objectId})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "models.Recipe has been deleted"})
}

// SearchRecipeHandler busca recetas por etiqueta
func SearchRecipeHandler(c *gin.Context) {
	// Obtener parámetro 'tag' de la query string
	tag := c.Query("tag")
	listOfRecipes := make([]models.Recipe, 0)
	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	filter := bson.M{"tags": bson.M{"$in": []string{tag}}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var recipe models.Recipe
	for cursor.Next(context.TODO()) {
		cursor.Decode(&recipe)
		listOfRecipes = append(listOfRecipes, recipe)
	}

	c.JSON(http.StatusOK, listOfRecipes)
}

// SearchRecipeByID busca una receta por su ID
func SearchRecipeByID(c *gin.Context) {
	id := c.Param("id")
	var recipe models.Recipe
	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	objectId, _ := primitive.ObjectIDFromHex(id)

	err := collection.FindOne(ctx, bson.M{"_id": objectId}).Decode(&recipe)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Devolver receta encontrada
	c.JSON(http.StatusOK, recipe)
}

// main configura el servidor HTTP y las rutas
func main() {
	// Crear enrutador Gin con middleware por defecto
	router := gin.Default()

	// Configurar rutas para el CRUD de recetas
	router.POST("/recipes", NewRecipeHandler)               // Crear
	router.GET("/recipes", recipeHandler.ListRecipeHandler) // Listar
	router.PUT("/recipes/:id", UpdateRecipeHandler)         // Actualizar
	router.DELETE("/recipes/:id", DeleteRecipeHandler)      // Eliminar
	router.GET("/recipes/search", SearchRecipeHandler)      // Buscar por tag
	router.GET("/recipes/:id", SearchRecipeByID)            // Obtener por ID

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
