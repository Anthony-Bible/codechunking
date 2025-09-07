package goparser

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
)

// ExtractGenericParameters extracts generic parameters from a Go parse tree.
func (p *GoParser) ExtractGenericParameters(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
) ([]outbound.GenericParameter, error) {
	slogger.Info(ctx, "Extracting Go generic parameters", slogger.Fields{})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	// Minimal implementation - return empty for now
	return []outbound.GenericParameter{}, nil
}

// parseGoGenericParameters parses generic type parameters.
func (p *GoParser) parseGoGenericParameters(
	parseTree *valueobject.ParseTree,
	typeParams *valueobject.ParseNode,
) []outbound.GenericParameter {
	var generics []outbound.GenericParameter

	typeParamDecls := findChildrenByType(typeParams, "type_parameter_declaration")
	for _, decl := range typeParamDecls {
		identifier := findChildByTypeInNode(decl, "type_identifier")
		if identifier != nil {
			name := parseTree.GetNodeText(identifier)
			constraints := []string{"any"} // Default constraint

			// Look for constraint
			constraint := findChildByTypeInNode(decl, "type_constraint")
			if constraint != nil {
				constraints = []string{parseTree.GetNodeText(constraint)}
			}

			generics = append(generics, outbound.GenericParameter{
				Name:        name,
				Constraints: constraints,
			})
		}
	}

	return generics
}

// parseGoReceiver parses method receiver.
func (p *GoParser) parseGoReceiver(
	parseTree *valueobject.ParseTree,
	receiver *valueobject.ParseNode,
) string {
	paramDecl := findChildByTypeInNode(receiver, "parameter_declaration")
	if paramDecl == nil {
		return ""
	}

	// Look for type in parameter declaration
	typeNode := findChildByTypeInNode(paramDecl, "pointer_type")
	if typeNode != nil {
		typeIdent := findChildByTypeInNode(typeNode, "type_identifier")
		if typeIdent != nil {
			return "*" + parseTree.GetNodeText(typeIdent)
		}
	}

	typeIdent := findChildByTypeInNode(paramDecl, "type_identifier")
	if typeIdent != nil {
		return parseTree.GetNodeText(typeIdent)
	}

	return ""
}

// HasGenericParameters checks if a node has generic parameters.
func (p *GoParser) HasGenericParameters(node *valueobject.ParseNode) bool {
	return findChildByTypeInNode(node, "type_parameter_list") != nil
}

// GetGenericParameterNames extracts generic parameter names.
func (p *GoParser) GetGenericParameterNames(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []string {
	typeParamList := findChildByTypeInNode(node, "type_parameter_list")
	if typeParamList == nil {
		return []string{}
	}

	var names []string
	typeParamDecls := findChildrenByType(typeParamList, "type_parameter_declaration")
	for _, decl := range typeParamDecls {
		identifier := findChildByTypeInNode(decl, "type_identifier")
		if identifier != nil {
			names = append(names, parseTree.GetNodeText(identifier))
		}
	}

	return names
}

// ParseConstraints extracts constraints for each generic parameter.
func (p *GoParser) ParseConstraints(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) map[string][]string {
	constraints := make(map[string][]string)
	typeParamList := findChildByTypeInNode(node, "type_parameter_list")
	if typeParamList == nil {
		return constraints
	}

	typeParamDecls := findChildrenByType(typeParamList, "type_parameter_declaration")
	for _, decl := range typeParamDecls {
		identifier := findChildByTypeInNode(decl, "type_identifier")
		if identifier != nil {
			name := parseTree.GetNodeText(identifier)
			constraint := findChildByTypeInNode(decl, "type_constraint")
			if constraint != nil {
				constraints[name] = []string{parseTree.GetNodeText(constraint)}
			} else {
				constraints[name] = []string{"any"}
			}
		}
	}

	return constraints
}

// ParseGoTypeReference parses a Go type reference.
func (p *GoParser) ParseGoTypeReference(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) *outbound.TypeReference {
	if node == nil {
		return nil
	}

	// Try to find a type identifier
	typeIdent := findChildByTypeInNode(node, "type_identifier")
	if typeIdent != nil {
		name := parseTree.GetNodeText(typeIdent)
		return &outbound.TypeReference{
			Name: name,
		}
	}

	// Try to find a pointer type
	pointerType := findChildByTypeInNode(node, "pointer_type")
	if pointerType != nil {
		typeIdent := findChildByTypeInNode(pointerType, "type_identifier")
		if typeIdent != nil {
			name := parseTree.GetNodeText(typeIdent)
			return &outbound.TypeReference{
				Name: "*" + name,
			}
		}
	}

	// Try to find an array type
	arrayType := findChildByTypeInNode(node, "array_type")
	if arrayType != nil {
		elementType := findChildByTypeInNode(arrayType, "type_identifier")
		if elementType != nil {
			name := parseTree.GetNodeText(elementType)
			return &outbound.TypeReference{
				Name: "[]" + name,
			}
		}
	}

	// Try to find a slice type
	sliceType := findChildByTypeInNode(node, "slice_type")
	if sliceType != nil {
		elementType := findChildByTypeInNode(sliceType, "type_identifier")
		if elementType != nil {
			name := parseTree.GetNodeText(elementType)
			return &outbound.TypeReference{
				Name: "[]" + name,
			}
		}
	}

	return nil
}
