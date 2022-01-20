//go:build js && wasm
// +build js,wasm

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"syscall/js"

	"github.com/lithammer/dedent"
	"github.com/norunners/vert"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/printer"
	"github.com/withastro/compiler/internal/transform"
	wasm_utils "github.com/withastro/compiler/internal_wasm/utils"
	"golang.org/x/net/html/atom"
)

var done chan bool

func main() {
	js.Global().Set("__astro_transform", Transform())
	// This ensures that the WASM doesn't exit early
	<-make(chan bool)
}

func jsString(j js.Value) string {
	if j.IsUndefined() || j.IsNull() {
		return ""
	}
	return j.String()
}

func jsBool(j js.Value) bool {
	if j.IsUndefined() || j.IsNull() {
		return false
	}
	return j.Bool()
}

func makeTransformOptions(options js.Value, hash string) transform.TransformOptions {
	filename := jsString(options.Get("sourcefile"))
	if filename == "" {
		filename = "<stdin>"
	}

	pathname := jsString(options.Get("pathname"))
	if pathname == "" {
		pathname = "<stdin>"
	}

	as := jsString(options.Get("as"))
	if as == "" {
		as = "document"
	}

	internalURL := jsString(options.Get("internalURL"))
	if internalURL == "" {
		internalURL = "astro/internal"
	}

	sourcemap := jsString(options.Get("sourcemap"))
	if sourcemap == "<boolean: true>" {
		sourcemap = "both"
	}

	site := jsString(options.Get("site"))
	if site == "" {
		site = "https://astro.build"
	}

	projectRoot := jsString(options.Get("projectRoot"))
	if projectRoot == "" {
		projectRoot = "."
	}

	staticExtraction := false
	if jsBool(options.Get("experimentalStaticExtraction")) {
		staticExtraction = true
	}

	preprocessStyle := options.Get("preprocessStyle")

	return transform.TransformOptions{
		As:               as,
		Scope:            hash,
		Filename:         filename,
		Pathname:         pathname,
		InternalURL:      internalURL,
		SourceMap:        sourcemap,
		Site:             site,
		ProjectRoot:      projectRoot,
		PreprocessStyle:  preprocessStyle,
		StaticExtraction: staticExtraction,
	}
}

type RawSourceMap struct {
	File           string   `js:"file"`
	Mappings       string   `js:"mappings"`
	Names          []string `js:"names"`
	Sources        []string `js:"sources"`
	SourcesContent []string `js:"sourcesContent"`
	Version        int      `js:"version"`
}

type HoistedScript struct {
	Code string `js:"code"`
	Src  string `js:"src"`
	Type string `js:"type"`
}

type TransformResult struct {
	Code    string          `js:"code"`
	Map     string          `js:"map"`
	CSS     []string        `js:"css"`
	Scripts []HoistedScript `js:"scripts"`
}

// This is spawned as a goroutine to preprocess style nodes using an async function passed from JS
func preprocessStyle(i int, style *astro.Node, transformOptions transform.TransformOptions, cb func()) {
	defer cb()
	if style.FirstChild == nil {
		return
	}
	attrs := wasm_utils.GetAttrs(style)
	data, _ := wasm_utils.Await(transformOptions.PreprocessStyle.(js.Value).Invoke(style.FirstChild.Data, attrs))
	// note: Rollup (and by extension our Astro Vite plugin) allows for "undefined" and "null" responses if a transform wishes to skip this occurrence
	if data[0].IsUndefined() || data[0].IsNull() {
		return
	}
	str := jsString(data[0].Get("code"))
	if str == "" {
		return
	}
	style.FirstChild.Data = str
}

func Transform() interface{} {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		source := jsString(args[0])
		hash := astro.HashFromSource(source)
		transformOptions := makeTransformOptions(js.Value(args[1]), hash)

		handler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			resolve := args[0]

			var doc *astro.Node

			if transformOptions.As == "document" {
				docNode, err := astro.Parse(strings.NewReader(source))
				doc = docNode
				if err != nil {
					fmt.Println(err)
				}
			} else if transformOptions.As == "fragment" {
				nodes, err := astro.ParseFragment(strings.NewReader(source), &astro.Node{
					Type:     astro.ElementNode,
					Data:     atom.Template.String(),
					DataAtom: atom.Template,
				})
				if err != nil {
					fmt.Println(err)
				}
				doc = &astro.Node{
					Type:                astro.DocumentNode,
					HydrationDirectives: make(map[string]bool),
				}
				for i := 0; i < len(nodes); i++ {
					n := nodes[i]
					doc.AppendChild(n)
				}
			}

			// Hoist styles and scripts to the top-level
			transform.ExtractStyles(doc)

			// Pre-process styles
			// Important! These goroutines need to be spawned from this file or they don't work
			var wg sync.WaitGroup
			if len(doc.Styles) > 0 {
				if transformOptions.PreprocessStyle.(js.Value).IsUndefined() != true {
					for i, style := range doc.Styles {
						wg.Add(1)
						i := i
						go preprocessStyle(i, style, transformOptions, wg.Done)
					}
				}
			}
			// Wait for all the style goroutines to finish
			wg.Wait()

			// Perform CSS and element scoping as needed
			transform.Transform(doc, transformOptions)

			css := []string{}
			scripts := []HoistedScript{}
			// Only perform static CSS extraction if the flag is passed in.
			if transformOptions.StaticExtraction {
				css_result := printer.PrintCSS(source, doc, transformOptions)
				for _, bytes := range css_result.Output {
					css = append(css, string(bytes))
				}

				// Append hoisted scripts
				for _, node := range doc.Scripts {
					src := astro.GetAttribute(node, "src")
					script := HoistedScript{
						Src:  "",
						Code: "",
						Type: "",
					}
					if src != nil {
						script.Type = "external"
						script.Src = src.Val
					} else if node.FirstChild != nil {
						script.Type = "inline"
						script.Code = dedent.Dedent(strings.TrimLeft(node.FirstChild.Data, "\r\n"))
					}
					scripts = append(scripts, script)
				}
			}

			result := printer.PrintToJS(source, doc, len(css), transformOptions)

			switch transformOptions.SourceMap {
			case "external":
				resolve.Invoke(createExternalSourceMap(source, result, css, &scripts, transformOptions))
				return nil
			case "both":
				resolve.Invoke(createBothSourceMap(source, result, css, &scripts, transformOptions))
				return nil
			case "inline":
				resolve.Invoke(createInlineSourceMap(source, result, css, &scripts, transformOptions))
				return nil
			}

			resolve.Invoke(vert.ValueOf(TransformResult{
				CSS:     css,
				Code:    string(result.Output),
				Map:     "",
				Scripts: scripts,
			}))

			return nil
		})
		defer handler.Release()

		// Create and return the Promise object
		promiseConstructor := js.Global().Get("Promise")
		return promiseConstructor.New(handler)
	})
}

func createSourceMapString(source string, result printer.PrintResult, transformOptions transform.TransformOptions) string {
	sourcesContent, _ := json.Marshal(source)
	sourcemap := RawSourceMap{
		Version:        3,
		Sources:        []string{transformOptions.Filename},
		SourcesContent: []string{string(sourcesContent)},
		Mappings:       string(result.SourceMapChunk.Buffer),
	}
	return fmt.Sprintf(`{
  "version": 3,
  "sources": ["%s"],
  "sourcesContent": [%s],
  "mappings": "%s",
  "names": []
}`, sourcemap.Sources[0], sourcemap.SourcesContent[0], sourcemap.Mappings)
}

func createExternalSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, transformOptions transform.TransformOptions) interface{} {
	return vert.ValueOf(TransformResult{
		CSS:     css,
		Code:    string(result.Output),
		Map:     createSourceMapString(source, result, transformOptions),
		Scripts: *scripts,
	})
}

func createInlineSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, transformOptions transform.TransformOptions) interface{} {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		CSS:     css,
		Code:    string(result.Output) + "\n" + inlineSourcemap,
		Map:     "",
		Scripts: *scripts,
	})
}

func createBothSourceMap(source string, result printer.PrintResult, css []string, scripts *[]HoistedScript, transformOptions transform.TransformOptions) interface{} {
	sourcemapString := createSourceMapString(source, result, transformOptions)
	inlineSourcemap := `//# sourceMappingURL=data:application/json;charset=utf-8;base64,` + base64.StdEncoding.EncodeToString([]byte(sourcemapString))
	return vert.ValueOf(TransformResult{
		CSS:     css,
		Code:    string(result.Output) + "\n" + inlineSourcemap,
		Map:     sourcemapString,
		Scripts: *scripts,
	})
}
