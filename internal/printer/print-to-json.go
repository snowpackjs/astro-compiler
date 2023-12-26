package printer

import (
	"fmt"
	"regexp"
	"strings"

	. "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/sourcemap"
	"github.com/withastro/compiler/internal/t"
	"github.com/withastro/compiler/internal/transform"
)

type ASTPosition struct {
	Start ASTPoint `json:"start,omitempty"`
	End   ASTPoint `json:"end,omitempty"`
}

type ASTPoint struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type ASTNode struct {
	Type       string      `json:"type"`
	Name       string      `json:"name"`
	Value      string      `json:"value,omitempty"`
	Attributes []ASTNode   `json:"attributes,omitempty"`
	Directives []ASTNode   `json:"directives,omitempty"`
	Children   []ASTNode   `json:"children,omitempty"`
	Position   ASTPosition `json:"position,omitempty"`

	// Attributes only
	Kind string `json:"kind,omitempty"`
	Raw  string `json:"raw,omitempty"`
}

func escapeForJSON(value string) string {
	backslash := regexp.MustCompile(`\\`)
	value = backslash.ReplaceAllString(value, `\\`)
	newlines := regexp.MustCompile(`\n`)
	value = newlines.ReplaceAllString(value, `\n`)
	doublequotes := regexp.MustCompile(`"`)
	value = doublequotes.ReplaceAllString(value, `\"`)
	r := regexp.MustCompile(`\r`)
	value = r.ReplaceAllString(value, `\r`)
	t := regexp.MustCompile(`\t`)
	value = t.ReplaceAllString(value, `\t`)
	f := regexp.MustCompile(`\f`)
	value = f.ReplaceAllString(value, `\f`)
	return value
}

func (n ASTNode) String() string {
	str := fmt.Sprintf(`{"type":"%s"`, n.Type)
	if n.Kind != "" {
		str += fmt.Sprintf(`,"kind":"%s"`, n.Kind)
	}
	if n.Name != "" {
		str += fmt.Sprintf(`,"name":"%s"`, escapeForJSON(n.Name))
	} else if n.Type == "fragment" {
		str += `,"name":""`
	}
	if n.Value != "" || n.Type == "attribute" {
		str += fmt.Sprintf(`,"value":"%s"`, escapeForJSON(n.Value))
	}
	if n.Raw != "" || n.Type == "attribute" {
		str += fmt.Sprintf(`,"raw":"%s"`, escapeForJSON(n.Raw))
	}
	if len(n.Attributes) > 0 {
		str += `,"attributes":[`
		for i, attr := range n.Attributes {
			str += attr.String()
			if i < len(n.Attributes)-1 {
				str += ","
			}
		}
		str += `]`
	} else if n.Type == "element" || n.Type == "component" || n.Type == "custom-element" || n.Type == "fragment" {
		str += `,"attributes":[]`
	}
	if len(n.Children) > 0 {
		str += `,"children":[`
		for i, node := range n.Children {
			str += node.String()
			if i < len(n.Children)-1 {
				str += ","
			}
		}
		str += `]`
	} else if n.Type == "element" || n.Type == "component" || n.Type == "custom-element" || n.Type == "fragment" {
		str += `,"children":[]`
	}
	if n.Position.Start.Line != 0 {
		str += `,"position":{`
		str += fmt.Sprintf(`"start":{"line":%d,"column":%d,"offset":%d}`, n.Position.Start.Line, n.Position.Start.Column, n.Position.Start.Offset)
		if n.Position.End.Line != 0 {
			str += fmt.Sprintf(`,"end":{"line":%d,"column":%d,"offset":%d}`, n.Position.End.Line, n.Position.End.Column, n.Position.End.Offset)
		}
		str += "}"
	}
	str += "}"
	return str
}

func PrintToJSON(sourcetext string, n *Node, opts t.ParseOptions) PrintResult {
	p := &printer{
		builder:    sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
		sourcetext: sourcetext,
	}
	root := ASTNode{}
	renderNode(p, &root, n, opts)
	doc := root.Children[0]
	return PrintResult{
		Output: []byte(doc.String()),
	}
}

func locToPoint(p *printer, loc loc.Loc) ASTPoint {
	offset := loc.Start
	info := p.builder.GetLineAndColumnForLocation(loc)
	line := info[0]
	column := info[1]

	return ASTPoint{
		Line:   line,
		Column: column,
		Offset: offset,
	}
}

func positionAt(p *printer, n *Node, opts t.ParseOptions) ASTPosition {
	if !opts.Position {
		return ASTPosition{}
	}

	if len(n.Loc) == 1 {
		s := n.Loc[0]
		start := locToPoint(p, s)

		return ASTPosition{
			Start: start,
		}
	}

	if len(n.Loc) == 2 {
		s := n.Loc[0]
		e := n.Loc[1]
		// `s` and `e` mark the start location of the tag name
		if n.Type == ElementNode {
			// this adjusts `e` to be the last index of the end tag for self-closing tags
			if s.Start == e.Start {
				e.Start = e.Start + len(n.Data) + 2
			} else {
				// this adjusts `e` to be the last index of the end tag for normally closed tags
				e.Start = e.Start + len(n.Data) + 1
			}

			if s.Start != 0 {
				// this adjusts `s` to be the first index of the element tag
				s.Start = s.Start - 1
			}
		}
		start := locToPoint(p, s)
		end := locToPoint(p, e)

		return ASTPosition{
			Start: start,
			End:   end,
		}
	}
	return ASTPosition{}
}

func attrPositionAt(p *printer, n *Attribute, opts t.ParseOptions) ASTPosition {
	if !opts.Position {
		return ASTPosition{}
	}

	k := n.KeyLoc
	start := locToPoint(p, k)

	return ASTPosition{
		Start: start,
	}
}

func renderNode(p *printer, parent *ASTNode, n *Node, opts t.ParseOptions) {
	isImplicit := false
	for _, a := range n.Attr {
		if transform.IsImplicitNodeMarker(a) {
			isImplicit = true
			break
		}
	}
	hasChildren := n.FirstChild != nil
	if isImplicit {
		if hasChildren {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				renderNode(p, parent, c, opts)
			}
		}
		return
	}
	var node ASTNode

	node.Position = positionAt(p, n, opts)

	if n.Type == ElementNode {
		if n.Expression {
			node.Type = "expression"
		} else {
			node.Name = n.Data
			if n.Component {
				node.Type = "component"
			} else if n.CustomElement {
				node.Type = "custom-element"
			} else if n.Fragment {
				node.Type = "fragment"
			} else {
				node.Type = "element"
			}

			for _, attr := range n.Attr {
				name := attr.Key
				if attr.Namespace != "" {
					name = fmt.Sprintf("%s:%s", attr.Namespace, attr.Key)
				}
				position := attrPositionAt(p, &attr, opts)
				raw := ""
				if attr.Type == QuotedAttribute || attr.Type == TemplateLiteralAttribute {
					start := attr.ValLoc.Start - 1
					end := attr.ValLoc.Start + len(attr.Val)

					char := p.sourcetext[start]
					if char == '=' {
						start += 1
					} else {
						end += 1
					}
					raw = strings.TrimSpace(p.sourcetext[start:end])
				}
				attrNode := ASTNode{
					Type:     "attribute",
					Kind:     attr.Type.String(),
					Position: position,
					Name:     name,
					Value:    attr.Val,
					Raw:      raw,
				}
				node.Attributes = append(node.Attributes, attrNode)
			}
		}
	} else {
		node.Type = n.Type.String()
		if n.Type == TextNode || n.Type == CommentNode || n.Type == DoctypeNode {
			node.Value = n.Data
		}
	}
	if n.Type == FrontmatterNode && hasChildren {
		node.Value = n.FirstChild.Data
	} else {
		if !isImplicit && hasChildren {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				renderNode(p, &node, c, opts)
			}
		}
	}

	parent.Children = append(parent.Children, node)
}
