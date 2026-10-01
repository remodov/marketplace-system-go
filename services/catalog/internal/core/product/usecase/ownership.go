package usecase

import (
	"github.com/remodov/marketplace-system-go/services/catalog/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/security"
)

func requireOwnership(product *aggregate.Product, requester security.Principal) error {
	if requester.IsAdmin() || product.OwnedBy(requester.Sub) {
		return nil
	}
	return apperr.NotFound("OWN_PRODUCT_REQUIRED", "Продукт не найден")
}
