package tera

import (
	"fmt"
	"strings"
	"testing"
)

var benchmarkString string
var benchmarkValue Value

const benchBigTableTemplate = `
<table>
{% for row in table %}
<tr>{% for col in row %}<td>{{ col }}</td>{% endfor %}</tr>
{% endfor %}
</table>
`

const benchTeamsTemplate = `
<html>
  <head>
    <title>{{ year }}</title>
  </head>
  <body>
    <h1>CSL {{ year }}</h1>
    <ul>
    {% for team in teams %}
      <li class="{% if loop.index0 == 0 %}champion{% endif %}">
      <b>{{ team.name }}</b>: {{ team.score }}
      </li>
    {% endfor %}
    </ul>
  </body>
</html>
`

const benchNoHtmlShort = `A paragraph without HTML characters that need to be escaped.`

const benchHtmlShort = `Here->An <<example>> of rust codefn foo(u: &u32) -> &u32 {u}`

const benchNoHtmlVeryLong = `
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Proin scelerisque eu urna in aliquet.
Phasellus ac nulla a urna sagittis consequat id quis est. Nullam eu ex eget erat accumsan dictum
ac lobortis urna. Etiam fermentum ut quam at dignissim. Curabitur vestibulum luctus tellus, sit
amet lobortis augue tempor faucibus. Nullam sed felis eget odio elementum euismod in sit amet massa.
Vestibulum sagittis purus sit amet eros auctor, sit amet pharetra purus dapibus. Donec ornare metus
vel dictum porta. Etiam ut nisl nisi. Nullam rutrum porttitor mi. Donec aliquam ac ipsum eget
hendrerit. Cras faucibus, eros ut pharetra imperdiet, est tellus aliquet felis, eget convallis
lacus ipsum eget quam. Vivamus orci lorem, maximus ac mi eget, bibendum vulputate massa. In
vestibulum dui hendrerit, vestibulum lacus sit amet, posuere erat. Vivamus euismod massa diam,
vulputate euismod lectus vestibulum nec. Donec sit amet massa magna. Nunc ipsum nulla, euismod
quis lacus at, gravida maximus elit. Duis tristique, nisl nullam.
    `

const benchHtmlVeryLong = `
    Lorem ipsum dolor sit amet, consectetur adipiscing elit. Mauris consequat tellus sit
    amet ornare fermentum. Etiam nec erat ante. In at metus a orci mollis scelerisque.
    Sed eget ultrices turpis, at sollicitudin erat. Integer hendrerit nec magna quis
    venenatis. Vivamus non dolor hendrerit, vulputate velit sed, varius nunc. Quisque
    in pharetra mi. Sed ullamcorper nibh malesuada commodo porttitor. Ut scelerisque
    sodales felis quis dignissim. Morbi aliquam finibus justo, sit amet consectetur
    mauris efficitur sit amet. Donec posuere turpis felis, eu lacinia magna accumsan
    quis. Fusce egestas lacus vel fermentum tincidunt. Phasellus a nulla eget lectus
    placerat commodo at eget nisl. Fusce cursus dui quis purus accumsan auctor.
    Donec iaculis felis quis metus consectetur porttitor.
<p>
    Etiam nibh mi, <b>accumsan</b> quis purus sed, posuere fermentum lorem. In pulvinar porta
    maximus. Fusce tincidunt lacinia tellus sit amet tincidunt. Aliquam lacus est, pulvinar
    non metus a, <b>facilisis</b> ultrices quam. Nulla feugiat leo in cursus eleifend. Suspendisse
    eget nisi ac justo sagittis interdum id a ipsum. Nulla mauris justo, scelerisque ac
    rutrum vitae, consequat vel ex.
</p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p></p>
<p>
    Sed sollicitudin <b>sem</b> mauris, at rutrum nibh egestas vel. Ut eu nisi tellus. Praesent dignissim
    orci elementum, mattis turpis eget, maximus ante. Suspendisse luctus eu felis a tempor. Morbi
    ac risus vitae sem molestie ullamcorper. Curabitur ligula augue, sollicitudin quis maximus vel,
    facilisis sed nibh. Aenean auctor magna sem, id rutrum metus convallis quis. Nullam non arcu
    dictum, lobortis erat quis, rhoncus est. Suspendisse venenatis, mi sed venenatis vehicula,
    tortor dolor egestas lectus, et efficitur turpis odio non augue. Integer velit sapien, dictum
    non egestas vitae, hendrerit sed quam. Phasellus a nunc eu erat varius imperdiet. Etiam id
    sollicitudin turpis, vitae molestie orci. Quisque ornare magna quis metus rhoncus commodo.
    Phasellus non mauris velit.
</p>
<p>
    Etiam dictum tellus ipsum, nec varius quam ornare vel. Cras vehicula diam nec sollicitudin
    ultricies. Pellentesque rhoncus sagittis nisl id facilisis. Nunc viverra convallis risus ut
    luctus. Aliquam vestibulum <b>efficitur massa</b>, id tempus nisi posuere a. Aliquam scelerisque
    elit justo. Nullam a ante felis. Cras vitae lorem eu nisi feugiat hendrerit. Maecenas vitae
    suscipit leo, lacinia dignissim lacus. Sed eget volutpat mi. In eu bibendum neque. Pellentesque
    finibus velit a fermentum rhoncus. Maecenas leo purus, eleifend eu lacus a, condimentum sagittis
    justo.
</p>`

const benchObjectText = `Meta
Before we get to the details, two important notes about the ownership system.
Rust has a focus on safety and speed. It accomplishes these goals through many ‘zero-cost abstractions’, which means that in Rust, abstractions cost as little as possible in order to make them work. The ownership system is a prime example of a zero cost abstraction. All of the analysis we’ll talk about in this guide is done at compile time. You do not pay any run-time cost for any of these features.
However, this system does have a certain cost: learning curve. Many new users to Rust experience something we like to call ‘fighting with the borrow checker’, where the Rust compiler refuses to compile a program that the author thinks is valid. This often happens because the programmer’s mental model of how ownership should work doesn’t match the actual rules that Rust implements. You probably will experience similar things at first. There is good news, however: more experienced Rust developers report that once they work with the rules of the ownership system for a period of time, they fight the borrow checker less and less.
With that in mind, let’s learn about borrowing.`

func BenchmarkTemplates(b *testing.B) {
	b.Run("big-context", func(b *testing.B) {
		objects := make([]map[string]any, 100)
		for i := range objects {
			o := map[string]any{}
			for _, name := range []string{"field_a", "field_b", "field_c", "field_d", "field_e", "field_f"} {
				o[name] = map[string]any{"i": i, "v": benchObjectText}
			}
			objects[i] = o
		}
		engine := New()
		mustAdd(b, engine, RawTemplate{"big_loop.html", `
{%- for object in objects -%}
{{ object.field_a.i }}
{%- if object.field_a.i > 2 -%}
{%- break -%}
{%- endif -%}
{%- endfor -%}
`})
		c := testContext(map[string]any{"objects": objects})
		checkRender(b, engine, "big_loop.html", c, "0123")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s, err := engine.Render("big_loop.html", c.clone())
			if err != nil {
				b.Fatal(err)
			}
			benchmarkString = s
		}
	})
	b.Run("big-table", func(b *testing.B) {
		table := make([][]int, 100)
		for i := range table {
			table[i] = make([]int, 100)
			for j := range table[i] {
				table[i][j] = j
			}
		}
		c := testContext(map[string]any{"table": table})
		engine := New()
		mustAdd(b, engine, RawTemplate{"big-table.html", benchBigTableTemplate})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s, err := engine.Render("big-table.html", c)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkString = s
		}
	})
	b.Run("teams", func(b *testing.B) {
		engine := New()
		mustAdd(b, engine, RawTemplate{"teams.html", benchTeamsTemplate})
		c := testContext(map[string]any{"year": 2015, "teams": []map[string]any{{"name": "Jiangsu", "score": 43}, {"name": "Beijing", "score": 27}, {"name": "Guangzhou", "score": 22}, {"name": "Shandong", "score": 12}}})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s, err := engine.Render("teams.html", c)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkString = s
		}
	})
	b.Run("realistic", func(b *testing.B) {
		engine, c := realisticBenchmark(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s, err := engine.Render("page.html", c)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkString = s
		}
	})
}

type benchmarkPage struct {
	Path         string          `json:"path"`
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	Content      string          `json:"content"`
	Permalink    string          `json:"permalink"`
	Draft        bool            `json:"draft"`
	GenerateFeed bool            `json:"generate_feed"`
	WordCount    int             `json:"word_count"`
	ReadingTime  int             `json:"reading_time"`
	Backlinks    []string        `json:"backlinks"`
	Pages        []benchmarkPage `json:"pages"`
}

func realisticPage(i int) benchmarkPage {
	backlinks := make([]string, 20)
	for j := range backlinks {
		backlinks[j] = fmt.Sprintf("/blog/post-%d/", j)
	}
	return benchmarkPage{fmt.Sprintf("/blog/post-%d/", i), fmt.Sprintf("Some blog post title number %d", i), strings.Repeat("A couple of sentences summarizing what this post is about. ", 4), strings.Repeat(fmt.Sprintf("<p>Paragraph %d of rendered HTML content with some length to it.</p>\n", i), 80), fmt.Sprintf("https://example.com/blog/post-%d/", i), false, true, 1200, 6, backlinks, []benchmarkPage{}}
}

type benchmarkSection struct {
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Permalink    string  `json:"permalink"`
	Path         string  `json:"path"`
	GenerateFeed bool    `json:"generate_feed"`
	Transparent  bool    `json:"transparent"`
	WordCount    int     `json:"word_count"`
	ReadingTime  int     `json:"reading_time"`
	Pages        []Value `json:"pages"`
}

func makeBenchmarkSection(pages []Value) benchmarkSection {
	return benchmarkSection{"Blog", "All the posts on this site", "https://example.com/blog/", "/blog/", true, false, 120000, 600, pages}
}
func BenchmarkValue(b *testing.B) {
	pages := make([]benchmarkPage, 1000)
	for i := range pages {
		pages[i] = realisticPage(i)
	}
	b.Run("from_serializable", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkValue = mustValue(b, pages[1])
		}
	})
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("from_serializable-nested-values-%d", n), func(b *testing.B) {
			values := make([]Value, n)
			for i := range values {
				values[i] = mustValue(b, pages[i])
			}
			section := makeBenchmarkSection(values)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkValue = mustValue(b, section)
			}
		})
	}
	b.Run("from_serializable-already-value", func(b *testing.B) {
		values := make([]Value, 100)
		for i := range values {
			values[i] = mustValue(b, pages[i])
		}
		v := mustValue(b, makeBenchmarkSection(values))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkValue = mustValue(b, v)
		}
	})
}
func BenchmarkEscapeHTML(b *testing.B) {
	for _, tc := range []struct{ name, plain, html string }{{"short", benchNoHtmlShort, benchHtmlShort}, {"long", benchNoHtmlVeryLong, benchHtmlVeryLong}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for _, input := range []string{tc.plain, tc.html} {
					var out strings.Builder
					if err := EscapeHTML(input, &out); err != nil {
						b.Fatal(err)
					}
					benchmarkString = out.String()
				}
			}
		})
	}
}
