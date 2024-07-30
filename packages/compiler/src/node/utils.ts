import type {
	CommentNode,
	ComponentNode,
	CustomElementNode,
	DoctypeNode,
	ElementNode,
	ExpressionNode,
	FragmentNode,
	FrontmatterNode,
	LiteralNode,
	Node,
	ParentNode,
	RootNode,
	TagLikeNode,
	TextNode,
} from '../shared/ast.js';

export type Visitor = (node: Node, parent?: ParentNode, index?: number) => void | Promise<void>;

function guard<Type extends Node>(type: string) {
	return (node: Node): node is Type => node.type === type;
}

export const is = {
	parent(node: Node): node is ParentNode {
		return Array.isArray((node as any).children);
	},
	literal(node: Node): node is LiteralNode {
		return typeof (node as any).value === 'string';
	},
	tag(node: Node): node is TagLikeNode {
		return (
			node.type === 'element' ||
			node.type === 'custom-element' ||
			node.type === 'component' ||
			node.type === 'fragment'
		);
	},
	whitespace(node: Node): node is TextNode {
		return node.type === 'text' && node.value.trim().length === 0;
	},
	root: guard<RootNode>('root'),
	element: guard<ElementNode>('element'),
	customElement: guard<CustomElementNode>('custom-element'),
	component: guard<ComponentNode>('component'),
	fragment: guard<FragmentNode>('fragment'),
	expression: guard<ExpressionNode>('expression'),
	text: guard<TextNode>('text'),
	doctype: guard<DoctypeNode>('doctype'),
	comment: guard<CommentNode>('comment'),
	frontmatter: guard<FrontmatterNode>('frontmatter'),
};

class Walker {
	constructor(private callback: Visitor) {}
	async visit(node: Node, parent?: ParentNode, index?: number): Promise<void> {
		await this.callback(node, parent, index);
		if (is.parent(node)) {
			const promises = [];
			for (let i = 0; i < node.children.length; i++) {
				const child = node.children[i];
				promises.push(this.callback(child, node as ParentNode, i));
			}
			await Promise.all(promises);
		}
	}
}

export function walk(node: ParentNode, callback: Visitor): void {
	const walker = new Walker(callback);
	walker.visit(node);
}

function serializeAttributes(node: TagLikeNode): string {
	let output = '';
	for (const attr of node.attributes) {
		output += ' ';
		switch (attr.kind) {
			case 'empty': {
				output += `${attr.name}`;
				break;
			}
			case 'expression': {
				output += `${attr.name}={${attr.value}}`;
				break;
			}
			case 'quoted': {
				output += `${attr.name}=${attr.raw}`;
				break;
			}
			case 'template-literal': {
				output += `${attr.name}=\`${attr.value}\``;
				break;
			}
			case 'shorthand': {
				output += `{${attr.name}}`;
				break;
			}
			case 'spread': {
				output += `{...${attr.name}}`;
				break;
			}
		}
	}
	return output;
}

export interface SerializeOptions {
	selfClose: boolean;
}
/** @deprecated Please use `SerializeOptions`  */
export type SerializeOtions = SerializeOptions;

export function serialize(root: Node, opts: SerializeOptions = { selfClose: true }): string {
	let output = '';
	function visitor(node: Node) {
		if (is.root(node)) {
			for (const child of node.children) {
				visitor(child);
			}
		} else if (is.frontmatter(node)) {
			output += `---${node.value}---\n\n`;
		} else if (is.comment(node)) {
			output += `<!--${node.value}-->`;
		} else if (is.expression(node)) {
			output += '{';
			for (const child of node.children) {
				visitor(child);
			}
			output += '}';
		} else if (is.literal(node)) {
			output += node.value;
		} else if (is.tag(node)) {
			output += `<${node.name}`;
			output += serializeAttributes(node);
			if (node.children.length === 0 && opts.selfClose) {
				output += ' />';
			} else {
				output += '>';
				for (const child of node.children) {
					visitor(child);
				}
				output += `</${node.name}>`;
			}
		}
	}
	visitor(root);
	return output;
}
