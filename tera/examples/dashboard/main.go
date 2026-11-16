// Command dashboard demonstrates components, recursive navigation, and table data.
package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/Keats/tera/tera"
)

//go:embed *.html
var templates embed.FS

func navigation(label, href, icon string, active bool, children ...map[string]any) map[string]any {
	return map[string]any{"label": label, "href": href, "icon": icon, "active": active, "children": children}
}

func main() {
	engine := tera.New()
	var sources []tera.RawTemplate
	for _, name := range []string{"base.html", "components.html", "dashboard.html"} {
		body, err := templates.ReadFile(name)
		if err != nil {
			log.Fatal(err)
		}
		sources = append(sources, tera.RawTemplate{Name: name, Content: string(body)})
	}
	if err := engine.AddRawTemplates(sources); err != nil {
		log.Fatal(err)
	}
	orders := []map[string]any{
		{"id": 1001, "customer": "Alice Smith", "total": 150.00, "status": "completed"},
		{"id": 1002, "customer": "Bob Jones", "total": 89.99, "status": "pending"},
		{"id": 1003, "customer": "Carol White", "total": 245.50, "status": "processing"},
	}
	products := []map[string]any{
		{"name": "Laptop Pro", "category": "Electronics", "price": 999.99, "stock": 15},
		{"name": "Wireless Mouse", "category": "Electronics", "price": 29.99, "stock": 150},
		{"name": "Cotton T-Shirt", "category": "Clothing", "price": 19.99, "stock": 200},
	}
	customers := []map[string]any{{"name": "Alice Smith", "email": "alice@example.com"}, {"name": "Bob Jones", "email": "bob@example.com"}}
	notifications := []map[string]any{
		{"title": "New Order", "message": "You have 3 new orders waiting for review.", "type": "info", "dismissible": true},
		{"title": "Low Stock", "message": "5 products are running low on stock.", "type": "warning", "dismissible": true},
	}
	productRows := make([][]string, len(products))
	for i, p := range products {
		stock := p["stock"].(int)
		status := "Low Stock"
		if stock > 50 {
			status = "In Stock"
		}
		productRows[i] = []string{p["name"].(string), p["category"].(string), fmt.Sprintf("$%.2f", p["price"]), fmt.Sprint(stock), status}
	}
	navItems := []map[string]any{
		navigation("Dashboard", "/dashboard", "home", true),
		navigation("Products", "/products", "box", false,
			navigation("All Products", "/products", "", false),
			navigation("Categories", "/products/categories", "", false,
				navigation("Electronics", "/products/categories/electronics", "", false),
				navigation("Clothing", "/products/categories/clothing", "", false))),
		navigation("Orders", "/orders", "shopping-cart", false,
			navigation("Pending", "/orders?status=pending", "", false),
			navigation("Completed", "/orders?status=completed", "", false)),
		navigation("Customers", "/customers", "users", false),
		navigation("Settings", "/settings", "cog", false),
	}
	c := tera.NewContext()
	for k, v := range map[string]any{"site_name": "Acme Dashboard", "current_year": "2025", "orders": orders, "products": products, "customers": customers, "notifications": notifications, "product_rows": productRows, "nav_items": navItems, "total_revenue": 12500, "selected_count": 0, "pending_count": 5, "processing_count": 3, "shipped_count": 12, "delivered_count": 45, "cancelled_count": 2} {
		if err := c.Insert(k, v); err != nil {
			log.Fatal(err)
		}
	}
	if err := engine.RenderTo("dashboard.html", c, os.Stdout); err != nil {
		log.Fatal(err)
	}
	if _, err := fmt.Println(); err != nil {
		log.Fatal(err)
	}
}
