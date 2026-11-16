// Command big_table renders the corresponding Tera example.
package main

import (
	"fmt"
	"github.com/Keats/tera/tera"
	"log"
)

const source = `
<table>
{% for row in table %}
<tr>{% for col in row %}<td>{{ col }}</td>{% endfor %}</tr>
{% endfor %}
</table>
`

func main() {
	engine := tera.New()
	if err := engine.AddRawTemplate("big-table.html", source); err != nil {
		log.Fatal(err)
	}
	c := tera.NewContext()
	table := make([][]int, 100)
	for i := range table {
		table[i] = make([]int, 100)
		for j := range table[i] {
			table[i][j] = j
		}
	}
	if err := c.Insert("table", table); err != nil {
		log.Fatal(err)
	}
	_, err := engine.Render("big-table.html", c)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := fmt.Println(true); err != nil {
		log.Fatal(err)
	}
}
