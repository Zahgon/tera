// Command teams renders the corresponding Tera example.
package main

import (
	"flag"
	"fmt"
	"github.com/Keats/tera/tera"
	"log"
)

const source = `
<html>
  <head>
    <title>{{ title }} - {{ year }}</title>
  </head>
  <body>
    <h1>{{ league_name }} {{ year }}</h1>
    <p>Total teams: {{ teams | length }}</p>
    <table>
      <thead>
        <tr><th>#</th><th>Team</th><th>City</th><th>Founded</th><th>Score</th><th>W</th><th>L</th></tr>
      </thead>
      <tbody>
      {% for team in teams %}
        <tr class="{% if loop.first %}champion{% elif loop.last %}relegated{% else %}normal{% endif %} {% if loop.index0 % 2 == 0 %}even{% else %}odd{% endif %}">
          <td>{{ loop.index }}</td>
          <td><b>{{ team.name }}</b></td>
          <td>{{ team.city }}</td>
          <td>{{ team.founded }}</td>
          <td>{{ team.score }}</td>
          <td>{{ team.wins }}</td>
          <td>{{ team.losses }}</td>
        </tr>
      {% endfor %}
      </tbody>
    </table>
    <footer>
      <p>Season {{ year }} - {{ league_name }}</p>
    </footer>
  </body>
</html>
`

func main() {
	engine := tera.New()
	if err := engine.AddRawTemplate("teams.html", source); err != nil {
		log.Fatal(err)
	}
	c := tera.NewContext()
	iterations := flag.Int("iterations", 100000, "number of renders for the workload")
	flag.Parse()
	teams := make([]map[string]any, 50)
	for i := range teams {
		teams[i] = map[string]any{"name": fmt.Sprintf("Team %d", i), "score": 100 - i*2, "city": fmt.Sprintf("City %d", i%10), "founded": 1900 + i, "wins": 30 - i/2, "losses": i / 2}
	}
	if err := c.Insert("year", 2015); err != nil {
		log.Fatal(err)
	}
	if err := c.Insert("title", "Football League Standings"); err != nil {
		log.Fatal(err)
	}
	if err := c.Insert("league_name", "Premier League"); err != nil {
		log.Fatal(err)
	}
	if err := c.Insert("teams", teams); err != nil {
		log.Fatal(err)
	}
	for i := 0; i < *iterations; i++ {
		if _, err := engine.Render("teams.html", c); err != nil {
			log.Fatal(err)
		}
	}
}
