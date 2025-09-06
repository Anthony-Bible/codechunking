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

	typeParamDecls := p.findChildrenByType(typeParams, "type_parameter_declaration")
	for _, decl := range typeParamDecls {
		identifier := p.findChildByType(decl, "type_identifier")
		if identifier != nil {
			name := parseTree.GetNodeText(identifier)
			constraints := []string{"any"} // Default constraint

			// Look for constraint
			constraint := p.findChildByType(decl, "type_constraint")
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
	paramDecl := p.findChildByType(receiver, "parameter_declaration")
	if paramDecl == nil {
		return ""
	}

	// Look for type in parameter declaration
	typeNode := p.findChildByType(paramDecl, "pointer_type")
	if typeNode != nil {
		typeIdent := p.findChildByType(typeNode, "type_identifier")
		if typeIdent != nil {
			return "*" + parseTree.GetNodeText(typeIdent)
		}
	}

	typeIdent := p.findChildByType(paramDecl, "type_identifier")
	if typeIdent != nil {
		return parseTree.GetNodeText(typeIdent)
	}

	return ""
}

// HasGenericParameters checks if a node has generic parameters.
func (p *GoParser) HasGenericParameters(node *valueobject.ParseNode) bool {
	return p.findChildByType(node, "type_parameter_list") != nil
}

// GetGenericParameterNames extracts generic parameter names.
func (p *GoParser) GetGenericParameterNames(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []string {
	typeParamList := p.findChildByType(node, "type_parameter_list")
	if typeParamList == nil {
		return []string{}
	}

	var names []string
	typeParamDecls := p.findChildrenByType(typeParamList, "type_parameter_declaration")
	for _, decl := range typeParamDecls {
		identifier := p.findChildByType(decl, "type_identifier")
		if identifier != nil {
			names = append(names, parseTree.GetNodeText(identifier))
		}
	}

	return names
}

// ParseConstraints extracts constraints for each generic parameter.
func (p *GoParser) ParseConstraints(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) map[string][]string {
	constraints := make(map[string][]string)
	typeParamList := p.findChildByType(node, "type_parameter_list")
	if typeParamList == nil {
		return constraints
	}

	typeParamDecls := p.findChildrenByType(typeParamList, "type_parameter_declaration")
	for _, decl := range typeParamDecls {
		identifier := p.findChildByType(decl, "type_identifier")
		if identifier != nil {
			name := parseTree.GetNodeText(identifier)
			constraint := p.findChildByType(decl, "type_constraint")
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
	typeIdent := p.findChildByType(node, "type_identifier")
	if typeIdent != nil {
		name := parseTree.GetNodeText(typeIdent)
		return &outbound.TypeReference{
			Name: name,
		}
	}

	// Try to find a pointer type
	pointerType := p.findChildByType(node, "pointer_type")
	if pointerType != nil {
		typeIdent := p.findChildByType(pointerType, "type_identifier")
		if typeIdent != nil {
			name := parseTree.GetNodeText(typeIdent)
			return &outbound.TypeReference{
				Name: "*" + name,
			}
		}
	}

	// Try to find an array type
	arrayType := p.findChildByType(node, "array_type")
	if arrayType != nil {
		elementType := p.findChildByType(arrayType, "type_identifier")
		if elementType != nil {
			name := parseTree.GetNodeText(elementType)
			return &outbound.TypeReference{
				Name: "[]" + name,
			}
		}
	}

	// Try to find a slice type
	sliceType := p.findChildByType(node, "slice_type")
	if sliceType != nil {
		elementType := p.findChildByType(sliceType, "type_identifier")
		if elementType != nil {
			name := parseTree.GetNodeText(elementType)
			return &outbound.TypeReference{
				Name: "[]" + name,
			}
		}
	}

	return nil
}
