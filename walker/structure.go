// Package walker traverses a CSSTree AST in document or reverse order with enter and leave callbacks and a
// context of the enclosing nodes, and finds nodes; it also holds the structure of every node type.
package walker

// FieldType is one allowed value of a node field: Kind "string", "number", "boolean", "null", "node" (Node names
// the node type) or "list" (List names the allowed child node types; empty allows any node).
type FieldType struct {
	Kind string
	Node string
	List []string
}

// Field is a node field with its allowed values in the original's order.
type Field struct {
	Name  string
	Types []FieldType
}

// NodeType is the structure of a node type and the walker context field it sets.
type NodeType struct {
	Name        string
	Fields      []Field
	WalkContext string
}

var (
	str     = FieldType{Kind: "string"}
	boolean = FieldType{Kind: "boolean"}
	null    = FieldType{Kind: "null"}
)

func node(name string) FieldType     { return FieldType{Kind: "node", Node: name} }
func list(types ...string) FieldType { return FieldType{Kind: "list", List: types} }
func field(name string, types ...FieldType) Field {
	return Field{Name: name, Types: types}
}

var valueNodes = []FieldType{node("Identifier"), node("Number"), node("Dimension"), node("Ratio"), node("Function")}

// NodeTypes lists every node type in the order of the original's node index.
var NodeTypes = []NodeType{
	{"AnPlusB", []Field{field("a", str, null), field("b", str, null)}, ""},
	{"Atrule", []Field{field("name", str), field("prelude", node("AtrulePrelude"), node("Raw"), null),
		field("block", node("Block"), null)}, "atrule"},
	{"AtrulePrelude", []Field{field("children", list())}, "atrulePrelude"},
	{"AttributeSelector", []Field{field("name", node("Identifier")), field("matcher", str, null),
		field("value", node("String"), node("Identifier"), null), field("flags", str, null)}, ""},
	{"Block", []Field{field("children", list("Atrule", "Rule", "Declaration"))}, "block"},
	{"Brackets", []Field{field("children", list())}, ""},
	{"CDC", nil, ""},
	{"CDO", nil, ""},
	{"ClassSelector", []Field{field("name", str)}, ""},
	{"Combinator", []Field{field("name", str)}, ""},
	{"Comment", []Field{field("value", str)}, ""},
	{"Condition", []Field{field("kind", str), field("children",
		list("Identifier", "Feature", "FeatureFunction", "FeatureRange", "SupportsDeclaration"))}, ""},
	{"Declaration", []Field{field("important", boolean, str), field("property", str),
		field("value", node("Value"), node("Raw"))}, "declaration"},
	{"DeclarationList", []Field{field("children", list("Declaration", "Atrule", "Rule"))}, ""},
	{"Dimension", []Field{field("value", str), field("unit", str)}, ""},
	{"Feature", []Field{field("kind", str), field("name", str), field("value", append(valueNodes[:5:5], null)...)}, ""},
	{"FeatureFunction", []Field{field("kind", str), field("feature", str),
		field("value", node("Declaration"), node("Selector"))}, ""},
	{"FeatureRange", []Field{field("kind", str), field("left", valueNodes...), field("leftComparison", str),
		field("middle", valueNodes...), field("rightComparison", str, null),
		field("right", append(valueNodes[:5:5], null)...)}, ""},
	{"Function", []Field{field("name", str), field("children", list())}, "function"},
	{"GeneralEnclosed", []Field{field("kind", str), field("function", str, null), field("children", list())}, ""},
	{"Hash", []Field{field("value", str)}, ""},
	{"Identifier", []Field{field("name", str)}, ""},
	{"IdSelector", []Field{field("name", str)}, ""},
	{"Layer", []Field{field("name", str)}, ""},
	{"LayerList", []Field{field("children", list("Layer"))}, ""},
	{"MediaQuery", []Field{field("modifier", str, null), field("mediaType", str, null),
		field("condition", node("Condition"), null)}, ""},
	{"MediaQueryList", []Field{field("children", list("MediaQuery"))}, ""},
	{"NestingSelector", nil, ""},
	{"Nth", []Field{field("nth", node("AnPlusB"), node("Identifier")), field("selector", node("SelectorList"), null)}, ""},
	{"Number", []Field{field("value", str)}, ""},
	{"Operator", []Field{field("value", str)}, ""},
	{"Parentheses", []Field{field("children", list())}, ""},
	{"Percentage", []Field{field("value", str)}, ""},
	{"PseudoClassSelector", []Field{field("name", str), field("children", list("Raw"), null)}, "function"},
	{"PseudoElementSelector", []Field{field("name", str), field("children", list("Raw"), null)}, "function"},
	{"Ratio", []Field{field("left", node("Number"), node("Function")),
		field("right", node("Number"), node("Function"), null)}, ""},
	{"Raw", []Field{field("value", str)}, ""},
	{"Rule", []Field{field("prelude", node("SelectorList"), node("Raw")), field("block", node("Block"))}, "rule"},
	{"Scope", []Field{field("root", node("SelectorList"), node("Raw"), null),
		field("limit", node("SelectorList"), node("Raw"), null)}, ""},
	{"Selector", []Field{field("children", list("TypeSelector", "IdSelector", "ClassSelector",
		"AttributeSelector", "PseudoClassSelector", "PseudoElementSelector", "Combinator"))}, ""},
	{"SelectorList", []Field{field("children", list("Selector", "Raw"))}, "selector"},
	{"String", []Field{field("value", str)}, ""},
	{"StyleSheet", []Field{field("children", list("Comment", "CDO", "CDC", "Atrule", "Rule", "Raw"))}, "stylesheet"},
	{"SupportsDeclaration", []Field{field("declaration", node("Declaration"))}, ""},
	{"TypeSelector", []Field{field("name", str)}, ""},
	{"UnicodeRange", []Field{field("value", str)}, ""},
	{"Url", []Field{field("value", str)}, ""},
	{"Value", []Field{field("children", list())}, ""},
	{"WhiteSpace", []Field{field("value", str)}, ""},
}

// LookupNodeType returns the structure of a node type, or nil.
func LookupNodeType(name string) *NodeType {
	for i := range NodeTypes {
		if NodeTypes[i].Name == name {
			return &NodeTypes[i]
		}
	}
	return nil
}
