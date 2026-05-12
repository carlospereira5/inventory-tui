package loyverse

import (
	"fmt"
	"net/url"
)

// GetCategories obtiene todas las categorías de Loyverse con paginación.
func (c *Client) GetCategories() ([]Category, error) {
	var all []Category
	cursor := ""

	for {
		path := "/categories?limit=250"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}

		var resp CategoriesResponse
		if err := c.getJSON(path, &resp); err != nil {
			return nil, fmt.Errorf("fetching categories: %w", err)
		}

		all = append(all, resp.Categories...)

		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}

	return all, nil
}

// CreateItem crea un nuevo item en Loyverse y retorna el item creado con sus IDs asignados.
func (c *Client) CreateItem(req CreateItemRequest) (*LoyverseItem, error) {
	var item LoyverseItem
	if err := c.postJSON("/items", req, &item); err != nil {
		return nil, fmt.Errorf("creating item: %w", err)
	}
	return &item, nil
}
