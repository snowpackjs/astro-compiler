package printer

import (
	"fmt"
	"strings"

	astro "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/js_scanner"
	"github.com/snowpackjs/astro/internal/loc"
	"github.com/snowpackjs/astro/internal/sourcemap"
	"github.com/snowpackjs/astro/internal/transform"
	"golang.org/x/net/html/atom"
)

type PrintResult struct {
	Output         []byte
	SourceMapChunk sourcemap.Chunk
}

type printer struct {
	opts               transform.TransformOptions
	output             []byte
	builder            sourcemap.ChunkBuilder
	hasFuncPrelude     bool
	hasInternalImports bool
}

var TEMPLATE_TAG = "$$render"
var CREATE_ASTRO = "$$createAstro"
var CREATE_COMPONENT = "$$createComponent"
var RENDER_COMPONENT = "$$renderComponent"
var RENDER_SLOT = "$$renderSlot"
var ADD_ATTRIBUTE = "$$addAttribute"
var SPREAD_ATTRIBUTES = "$$spreadAttributes"
var DEFINE_STYLE_VARS = "$$defineStyleVars"
var DEFINE_SCRIPT_VARS = "$$defineScriptVars"
var CREATE_METADATA = "$$createMetadata"
var METADATA = "$$metadata"
var RESULT = "$$result"
var SLOTS = "$$slots"
var FRAGMENT = "Fragment"
var BACKTICK = "`"

func (p *printer) print(text string) {
	p.output = append(p.output, text...)
}

func (p *printer) println(text string) {
	p.output = append(p.output, (text + "\n")...)
}

func (p *printer) printInternalImports(importSpecifier string) {
	if p.hasInternalImports {
		return
	}
	p.print(fmt.Sprintf("import {\n  %s\n} from \"%s\";\n", strings.Join([]string{
		FRAGMENT,
		"render as " + TEMPLATE_TAG,
		"createAstro as " + CREATE_ASTRO,
		"createComponent as " + CREATE_COMPONENT,
		"renderComponent as " + RENDER_COMPONENT,
		"renderSlot as " + RENDER_SLOT,
		"addAttribute as " + ADD_ATTRIBUTE,
		"spreadAttributes as " + SPREAD_ATTRIBUTES,
		"defineStyleVars as " + DEFINE_STYLE_VARS,
		"defineScriptVars as " + DEFINE_SCRIPT_VARS,
		"createMetadata as " + CREATE_METADATA,
	}, ",\n  "), importSpecifier))
	p.hasInternalImports = true
}

func (p *printer) printReturnOpen() {
	p.addNilSourceMapping()
	p.print("return ")
	p.printTemplateLiteralOpen()
}

func (p *printer) printReturnClose() {
	p.addNilSourceMapping()
	p.printTemplateLiteralClose()
	p.println(";")
}

func (p *printer) printTemplateLiteralOpen() {
	p.addNilSourceMapping()
	p.print(fmt.Sprintf("%s%s", TEMPLATE_TAG, BACKTICK))
}

func (p *printer) printTemplateLiteralClose() {
	p.addNilSourceMapping()
	p.print(BACKTICK)
}

func (p *printer) printDefineVars(n *astro.Node) {
	// Only handle <script> or <style>
	if !(n.DataAtom == atom.Script || n.DataAtom == atom.Style) {
		return
	}
	for _, attr := range n.Attr {
		if attr.Key == "define:vars" {
			var value string
			var defineCall string

			if n.DataAtom == atom.Script {
				defineCall = DEFINE_SCRIPT_VARS
			} else if n.DataAtom == atom.Style {
				defineCall = DEFINE_STYLE_VARS
			}
			switch attr.Type {
			case astro.QuotedAttribute:
				value = `"` + attr.Val + `"`
			case astro.EmptyAttribute:
				value = attr.Key
			case astro.ExpressionAttribute:
				value = strings.TrimSpace(attr.Val)
			}
			p.addNilSourceMapping()
			p.print(fmt.Sprintf("${%s(", defineCall))
			p.addSourceMapping(attr.ValLoc)
			p.print(value)
			p.addNilSourceMapping()
			p.print(")}")
			return
		}
	}
}

func (p *printer) printFuncPrelude(componentName string) {
	if p.hasFuncPrelude {
		return
	}
	p.addNilSourceMapping()
	p.println("\n//@ts-ignore")
	p.println(fmt.Sprintf("const %s = %s(async (%s, $$props, %s) => {", componentName, CREATE_COMPONENT, RESULT, SLOTS))
	p.println(fmt.Sprintf("const Astro = %s.createAstro($$Astro, $$props, %s);", RESULT, SLOTS))
	p.hasFuncPrelude = true
}

func (p *printer) printFuncSuffix(componentName string) {
	p.addNilSourceMapping()
	p.println("});")
	p.println(fmt.Sprintf("export default %s;", componentName))
}

func (p *printer) printAttributesToObject(n *astro.Node) {
	p.print("{")
	for i, a := range n.Attr {
		if i != 0 {
			p.print(",")
		}
		switch a.Type {
		case astro.QuotedAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + a.Key + `"`)
			p.print(":")
			p.addSourceMapping(a.ValLoc)
			p.print(`"` + a.Val + `"`)
		case astro.EmptyAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + a.Key + `"`)
			p.print(":")
			p.print("true")
		case astro.ExpressionAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + a.Key + `"`)
			p.print(":")
			p.addSourceMapping(a.ValLoc)
			p.print(`(` + a.Val + `)`)
		case astro.SpreadAttribute:
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 3})
			p.print(`...(` + strings.TrimSpace(a.Key) + `)`)
		case astro.ShorthandAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + strings.TrimSpace(a.Key) + `"`)
			p.print(":")
			p.addSourceMapping(a.KeyLoc)
			p.print(`(` + strings.TrimSpace(a.Key) + `)`)
		case astro.TemplateLiteralAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + strings.TrimSpace(a.Key) + `"`)
			p.print(":")
			p.print("`" + strings.TrimSpace(a.Key) + "`")
		}
	}
	p.print("}")
}

func (p *printer) printStyleOrScript(n *astro.Node) {
	p.addNilSourceMapping()
	p.print("{props:")
	p.printAttributesToObject(n)
	if n.FirstChild != nil && strings.TrimSpace(n.FirstChild.Data) != "" {
		p.print(",children:`")
		p.addSourceMapping(n.Loc[0])
		p.print(escapeText(strings.TrimSpace(n.FirstChild.Data)))
		p.addNilSourceMapping()
		p.print("`")
	}
	p.print("},\n")
}

func (p *printer) printAttribute(attr astro.Attribute) {
	if attr.Key == "define:vars" {
		return
	}

	if attr.Namespace != "" {
		p.print(attr.Namespace)
		p.print(":")
	}

	switch attr.Type {
	case astro.QuotedAttribute:
		p.print(" ")
		p.addSourceMapping(attr.KeyLoc)
		p.print(attr.Key)
		p.print("=")
		p.addSourceMapping(attr.ValLoc)
		p.print(`"` + attr.Val + `"`)
	case astro.EmptyAttribute:
		p.print(" ")
		p.addSourceMapping(attr.KeyLoc)
		p.print(attr.Key)
	case astro.ExpressionAttribute:
		p.print(fmt.Sprintf("${%s(", ADD_ATTRIBUTE))
		p.addSourceMapping(attr.ValLoc)
		p.print(strings.TrimSpace(attr.Val))
		p.addSourceMapping(attr.KeyLoc)
		p.print(`, "` + strings.TrimSpace(attr.Key) + `")}`)
	case astro.SpreadAttribute:
		p.print(fmt.Sprintf("${%s(", SPREAD_ATTRIBUTES))
		p.addSourceMapping(loc.Loc{Start: attr.KeyLoc.Start - 3})
		p.print(strings.TrimSpace(attr.Key))
		p.print(`, "` + strings.TrimSpace(attr.Key) + `")}`)
	case astro.ShorthandAttribute:
		p.print(fmt.Sprintf("${%s(", ADD_ATTRIBUTE))
		p.addSourceMapping(attr.KeyLoc)
		p.print(strings.TrimSpace(attr.Key))
		p.addSourceMapping(attr.KeyLoc)
		p.print(`, "` + strings.TrimSpace(attr.Key) + `")}`)
	case astro.TemplateLiteralAttribute:
		p.print(fmt.Sprintf("${%s(`", ADD_ATTRIBUTE))
		p.addSourceMapping(attr.ValLoc)
		p.print(strings.TrimSpace(attr.Val))
		p.addSourceMapping(attr.KeyLoc)
		p.print("`" + `, "` + strings.TrimSpace(attr.Key) + `")}`)
	}
}

func (p *printer) addSourceMapping(location loc.Loc) {
	p.builder.AddSourceMapping(location, p.output)
}

func (p *printer) addNilSourceMapping() {
	p.builder.AddSourceMapping(loc.Loc{Start: 0}, p.output)
}

func (p *printer) printTopLevelAstro() {
	p.println(fmt.Sprintf("const $$Astro = %s(import.meta.url, '%s');\nconst Astro = $$Astro;", CREATE_ASTRO, p.opts.Site))
}

func (p *printer) printComponentMetadata(doc *astro.Node, source []byte) {
	var specs []string

	modCount := 1
	loc, specifier := js_scanner.NextImportSpecifier(source, 0)
	for loc != -1 {
		p.print(fmt.Sprintf("\nimport * as $$module%v from '%s';", modCount, specifier))
		specs = append(specs, specifier)
		loc, specifier = js_scanner.NextImportSpecifier(source, loc)
		modCount++
	}
	// If we added imports, add a line break.
	if modCount > 1 {
		p.print("\n")
	}

	// Call createMetadata
	p.print(fmt.Sprintf("\nexport const $$metadata = %s(import.meta.url, { ", CREATE_METADATA))

	// Add modules
	p.print("modules: [")
	for i := 1; i < modCount; i++ {
		if i > 1 {
			p.print(", ")
		}
		p.print(fmt.Sprintf("{ module: $$module%v, specifier: '%s' }", i, specs[i-1]))
	}
	p.print("]")

	// Hydrated Components
	p.print(", hydratedComponents: [")
	for i, node := range doc.Metadata.HydratedComponents {
		if i > 0 {
			p.print(", ")
		}

		if node.CustomElement {
			p.print(fmt.Sprintf("'%s'", node.Data))
		} else {
			p.print(node.Data)
		}
	}
	p.print("], hoisted: [")
	for i, node := range doc.Scripts {
		if i > 0 {
			p.print(", ")
		}

		src := astro.GetAttribute(node, "src")
		if src != nil {
			p.print(fmt.Sprintf("{ type: 'remote', src: '%s' }", escapeSingleQuote(src.Val)))
		} else if node.FirstChild != nil {
			p.print(fmt.Sprintf("{ type: 'inline', value: '%s' }", escapeSingleQuote(node.FirstChild.Data)))
		}
	}
	p.print("]")

	// Add Resources (<link> tags)
	p.print(", resources: [")
	for tagI, tagAttrs := range doc.Metadata.Resources {
		if tagI > 0 {
			p.print(",")
		}

		p.print("{")
		for attrI, attr := range tagAttrs {
			if attrI > 0 {
				p.print(",")
			}
			switch attr.Type {
			// href
			case astro.EmptyAttribute:
				p.print(fmt.Sprintf("'%s':true", escapeSingleQuote(attr.Key)))
			// href={styleURL}
			case astro.ExpressionAttribute:
				p.print(fmt.Sprintf("'%s':%s", escapeSingleQuote(attr.Key), attr.Val))
			// href="styles.css"
			case astro.QuotedAttribute:
				p.print(fmt.Sprintf("'%s':'%s'", escapeSingleQuote(attr.Key), escapeSingleQuote(attr.Val)))
			// {...props}
			case astro.SpreadAttribute:
				p.print(fmt.Sprintf("...(%s)", attr.Key))
			// {value}
			case astro.ShorthandAttribute:
				p.print(fmt.Sprintf("[%s]:true", attr.Key))
			// href=`styles.css`
			case astro.TemplateLiteralAttribute:
				p.print(fmt.Sprintf("'%s':`%s`", escapeSingleQuote(attr.Key), attr.Val))
			}
		}
		p.print("}")
	}
	p.print("] });\n\n")
}
