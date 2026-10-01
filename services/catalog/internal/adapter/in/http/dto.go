package http

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
)

func init() {
	decimal.MarshalJSONWithoutQuotes = true
}

type ProductDTO struct {
	ID          uuid.UUID       `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Price       decimal.Decimal `json:"price"`
	Currency    string          `json:"currency"`
	SellerID    uuid.UUID       `json:"sellerId"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type ProductPageDTO struct {
	Items []ProductDTO `json:"items"`
	Page  int          `json:"page"`
	Size  int          `json:"size"`
	Total int64        `json:"total"`
}

type CreateProductRequest struct {
	Title       *string          `json:"title"`
	Description string           `json:"description"`
	Price       *decimal.Decimal `json:"price"`
	Currency    *string          `json:"currency"`
}

type ChangePriceRequest struct {
	Price *decimal.Decimal `json:"price"`
}

func toDTO(p *aggregate.Product) ProductDTO {
	return ProductDTO{
		ID: p.ID(), Title: p.Title(), Description: p.Description(), Price: p.Price(), Currency: p.Currency(),
		SellerID: p.SellerID(), Status: string(p.Status()), CreatedAt: p.CreatedAt(), UpdatedAt: p.UpdatedAt(),
	}
}

func toPageDTO(page out.ProductPage) ProductPageDTO {
	items := make([]ProductDTO, 0, len(page.Items))
	for _, p := range page.Items {
		items = append(items, toDTO(p))
	}
	return ProductPageDTO{Items: items, Page: page.Page, Size: page.Size, Total: page.Total}
}
