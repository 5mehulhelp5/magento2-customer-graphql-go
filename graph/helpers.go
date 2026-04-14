package graph

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/magendooro/magento2-customer-graphql-go/graph/model"
	"github.com/magendooro/magento2-customer-graphql-go/internal/repository"
)

// decodeUID decodes a base64-encoded UID to an integer ID.
func decodeUID(uid string) (int, error) {
	decoded, err := base64.StdEncoding.DecodeString(uid)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(decoded))
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func coalesce(values ...*string) *string {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

// mapWishlistItem converts a WishlistItemData from the repository to the GraphQL model.
func mapWishlistItem(item *repository.WishlistItemData) *model.WishlistItem {
	id := strconv.Itoa(item.ItemID)

	var thumbnailURL *string
	if item.Thumbnail != "" {
		u := fmt.Sprintf("/media/catalog/product%s", item.Thumbnail)
		thumbnailURL = &u
	}
	label := item.Name

	priceVal := item.Price
	price := &model.Money{Value: &priceVal}

	return &model.WishlistItem{
		ID:       id,
		Quantity: item.Qty,
		AddedAt:  item.AddedAt,
		Product: &model.WishlistItemProduct{
			Sku:    item.SKU,
			Name:   item.Name,
			URLKey: item.URLKey,
			Thumbnail: &model.WishlistItemThumbnail{
				URL:   thumbnailURL,
				Label: &label,
			},
			PriceRange: &model.WishlistItemPriceRange{
				MinimumPrice: &model.WishlistItemMinPrice{
					FinalPrice: price,
				},
			},
		},
	}
}
