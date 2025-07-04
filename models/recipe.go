package models

import (
	"time"
)

// Recipe define la estructura de una receta culinaria
type Recipe struct {
	ID           string    `json:"id" bson:"_id"`                    // ID único generado con xid
	Name         string    `json:"name" bson:"name"`                 // Nombre de la receta (ej: "Pasta Carbonara")
	Tags         []string  `json:"tags" bson:"tags"`                 // Categorías (ej: ["italian", "pasta"])
	Ingredients  []string  `json:"ingredients" bson:"ingredients"`   // Lista de ingredientes necesarios
	Instructions []string  `json:"instructions" bson:"instructions"` // Pasos para preparar la receta
	PublishedAt  time.Time `json:"publishedAt" bson:"created"`       // Fecha de creación/actualización
}
