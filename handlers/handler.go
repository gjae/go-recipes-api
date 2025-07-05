package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjae/go-recipes-api/models"
	"github.com/gjae/go-recipes-api/utils"
	"github.com/go-redis/redis"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// RecipeHandler es una estructura que contiene las dependencias necesarias
// para manejar las operaciones relacionadas con recetas
type RecipeHandler struct {
	collection  *mongo.Collection // Conexión a la colección de MongoDB
	ctx         context.Context   // Contexto para operaciones de base de datos
	redisClient *redis.Client     // Cliente de Redis para caché
}

// NewRecipeHandler es el constructor para RecipeHandler
// Inicializa el handler con las dependencias inyectadas
func NewRecipeHandler(ctx context.Context, collection *mongo.Collection, redisClient *redis.Client) *RecipeHandler {
	return &RecipeHandler{
		collection:  collection,
		ctx:         ctx,
		redisClient: redisClient,
	}
}

// MakePing verifica la conexión con Redis
// (Nota: Sería útil que devolviera el estado en lugar de solo imprimirlo)
func (handler *RecipeHandler) MakePing() {
	status := handler.redisClient.Ping()
	fmt.Println("Redis status ... ", status)
}

// ListRecipeHandler maneja la solicitud para listar todas las recetas
// Implementa caché con Redis para mejorar el rendimiento
func (handler *RecipeHandler) ListRecipeHandler(c *gin.Context) {
	// Primero intenta obtener de Redis
	val, err := handler.redisClient.Get("recipes").Result()
	recipes := make([]models.Recipe, 0)

	if err == redis.Nil {
		// Si no hay datos en Redis, consulta MongoDB
		cur, err := handler.collection.Find(handler.ctx, bson.M{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		defer cur.Close(handler.ctx)

		// Itera sobre el cursor para obtener todas las recetas
		for cur.Next(handler.ctx) {
			var recipe models.Recipe
			cur.Decode(&recipe)
			recipes = append(recipes, recipe)
		}

		// Almacena en Redis para futuras consultas
		data, _ := json.Marshal(recipes)
		handler.redisClient.Set("recipes", string(data), 0)
	} else if err != nil {
		// Manejo de otros errores de Redis
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else {
		// Si hay datos en Redis, los decodifica
		log.Print("Request to redis")
		json.Unmarshal([]byte(val), &recipes)
	}

	c.JSON(http.StatusOK, recipes)
}

// NewRecipeHandler maneja la creación de una nueva receta
func (handler *RecipeHandler) NewRecipeHandler(c *gin.Context) {
	var recipe models.Recipe

	// Limpia la caché de recetas después de la operación
	defer func(redisClient *redis.Client) {
		go utils.CleanCacheById(redisClient, "recipes")
	}(handler.redisClient)

	// Parsea el JSON del request
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Asigna ID y fecha de publicación
	recipe.ID = primitive.NewObjectID()
	recipe.PublishedAt = time.Now()

	// Inserta en MongoDB
	_, err := handler.collection.InsertOne(handler.ctx, recipe)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, recipe)
}

// SearchRecipeByID busca una receta por su ID
// Utiliza Redis como caché para mejorar el rendimiento
func (handler *RecipeHandler) SearchRecipeByID(c *gin.Context) {
	id := c.Param("id")
	var recipe models.Recipe

	// Intenta obtener de Redis primero
	redisResult, err := handler.redisClient.Get(id).Result()

	if err == redis.Nil {
		// Si no está en Redis, busca en MongoDB
		objectId, _ := primitive.ObjectIDFromHex(id)

		err := handler.collection.FindOne(handler.ctx, bson.M{"_id": objectId}).Decode(&recipe)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Almacena en Redis para futuras consultas
		data, _ := json.Marshal(recipe)
		handler.redisClient.Set(id, data, 0)
	} else if err != nil {
		// Manejo de otros errores de Redis
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Decodifica el resultado de Redis
	json.Unmarshal([]byte(redisResult), &recipe)
	c.JSON(http.StatusOK, recipe)
}

// SearchRecipeHandler busca recetas por etiqueta
// (Nota: Podría beneficiarse de caché similar a otros handlers)
func (handler *RecipeHandler) SearchRecipeHandler(c *gin.Context) {
	tag := c.Query("tag")
	listOfRecipes := make([]models.Recipe, 0)

	// Crea filtro para buscar recetas que contengan el tag
	filter := bson.M{"tags": bson.M{"$in": []string{tag}}}

	cursor, err := handler.collection.Find(handler.ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(handler.ctx)

	// Itera sobre los resultados
	var recipe models.Recipe
	for cursor.Next(context.TODO()) {
		cursor.Decode(&recipe)
		listOfRecipes = append(listOfRecipes, recipe)
	}

	c.JSON(http.StatusOK, listOfRecipes)
}

// DeleteRecipeHandler elimina una receta
// (Nota: Hay un typo en el nombre original "DeleteRecippeHandler")
func (handler *RecipeHandler) DeleteRecipeHandler(c *gin.Context) {
	id := c.Param("id")

	// Limpia la caché después de eliminar
	defer func() {
		go utils.CleanCacheById(handler.redisClient, id)
	}()

	log.Printf("Deleting recipe ID # %s #\n", id)

	objectId, _ := primitive.ObjectIDFromHex(id)
	_, err := handler.collection.DeleteOne(handler.ctx, bson.M{"_id": objectId})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Recipe ID # %s # was deleted\n", id)
	c.JSON(http.StatusOK, gin.H{"message": "models.Recipe has been deleted"})
}

// UpdateRecipeHandler actualiza una receta existente
func (handler *RecipeHandler) UpdateRecipeHandler(c *gin.Context) {
	id := c.Param("id")

	// Limpia la caché después de actualizar
	defer func() {
		go utils.CleanCacheById(handler.redisClient, id)
	}()

	var recipe models.Recipe
	objectId, _ := primitive.ObjectIDFromHex(id)

	// Parsea el JSON del request
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Datos de receta inválidos: " + err.Error(),
		})
		return
	}

	// Actualiza en MongoDB
	_, err := handler.collection.UpdateOne(handler.ctx, bson.M{
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
