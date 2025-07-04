package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjae/go-recipes-api/models"
	"go.mongodb.org/mongo-driver/bson"
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
