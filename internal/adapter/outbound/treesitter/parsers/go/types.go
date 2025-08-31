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
