package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjae/go-recipes-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RecipeHandler struct {
	collection *mongo.Collection
	ctx        context.Context
}

func NewRecipeHandler(ctx context.Context, collection *mongo.Collection) *RecipeHandler {
	return &RecipeHandler{
		collection: collection,
		ctx:        ctx,
	}
}

func (handler *RecipeHandler) ListRecipeHandler(c *gin.Context) {
	cur, err := handler.collection.Find(handler.ctx, bson.M{})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	defer cur.Close(handler.ctx)

	recipes := make([]models.Recipe, 0)

	for cur.Next(handler.ctx) {
		var recipe models.Recipe
		cur.Decode(&recipe)
		recipes = append(recipes, recipe)
	}

	c.JSON(http.StatusOK, recipes)
}

func (handler *RecipeHandler) NewRecipeHandler(c *gin.Context) {
	var recipe models.Recipe

	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	recipe.ID = primitive.NewObjectID()
	recipe.PublishedAt = time.Now()

	_, err := handler.collection.InsertOne(handler.ctx, recipe)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, recipe)
}

// SearchRecipeByID busca una receta por su ID
func (handler *RecipeHandler) SearchRecipeByID(c *gin.Context) {
	id := c.Param("id")
	var recipe models.Recipe
	objectId, _ := primitive.ObjectIDFromHex(id)

	err := handler.collection.FindOne(handler.ctx, bson.M{"_id": objectId}).Decode(&recipe)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Devolver receta encontrada
	c.JSON(http.StatusOK, recipe)
}

// SearchRecipeHandler busca recetas por etiqueta
func (handler *RecipeHandler) SearchRecipeHandler(c *gin.Context) {
	// Obtener parámetro 'tag' de la query string
	tag := c.Query("tag")
	listOfRecipes := make([]models.Recipe, 0)
	filter := bson.M{"tags": bson.M{"$in": []string{tag}}}

	cursor, err := handler.collection.Find(handler.ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(handler.ctx)

	var recipe models.Recipe
	for cursor.Next(context.TODO()) {
		cursor.Decode(&recipe)
		listOfRecipes = append(listOfRecipes, recipe)
	}

	c.JSON(http.StatusOK, listOfRecipes)
}

// DeleteRecipeHandler elimina una receta (Nota: Corregir nombre de función - typo en "DeleteRecippeHandler")
func (handler *RecipeHandler) DeleteRecipeHandler(c *gin.Context) {
	id := c.Param("id")

	objectId, _ := primitive.ObjectIDFromHex(id)
	_, err := handler.collection.DeleteOne(handler.ctx, bson.M{"_id": objectId})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "models.Recipe has been deleted"})
}

// UpdateRecipeHandler actualiza una receta existente
func (handler *RecipeHandler) UpdateRecipeHandler(c *gin.Context) {
	// Obtener ID de los parámetros de la URL
	id := c.Param("id")
	var recipe models.Recipe

	objectId, _ := primitive.ObjectIDFromHex(id)

	// Bindear JSON del request
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Datos de receta inválidos: " + err.Error(),
		})
		return
	}

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
