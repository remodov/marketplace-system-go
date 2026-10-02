package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/security"
)

type RequestImageUpload struct {
	ProductID   uuid.UUID
	Requester   security.Principal
	ContentType string
}

type RequestImageUploadHandler struct {
	products out.ProductRepository
	images   out.ImageStorage
	ids      out.IDGenerator
}

func NewRequestImageUploadHandler(products out.ProductRepository, images out.ImageStorage, ids out.IDGenerator) *RequestImageUploadHandler {
	return &RequestImageUploadHandler{products: products, images: images, ids: ids}
}

func (h *RequestImageUploadHandler) Handle(ctx context.Context, cmd RequestImageUpload) (out.PresignedUpload, error) {
	product, err := h.products.ByID(ctx, cmd.ProductID)
	if err != nil {
		return out.PresignedUpload{}, err
	}
	// TODO шаг 12: ссылку на загрузку получает только владелец товара (или администратор).
	// Карточку смотреть может кто угодно, а грузить в неё файлы - нет.
	// Чужой товар для не-владельца должен выглядеть как несуществующий.
	key := "products/" + product.ID().String() + "/" + h.ids.NewID().String()
	return h.images.PresignUpload(ctx, key, cmd.ContentType)
}
