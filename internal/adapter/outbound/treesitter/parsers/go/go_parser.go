package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// GoParser implements LanguageParser and GoLanguageParser for Go language parsing.
type GoParser struct {
	supportedLanguage valueobject.Language
}

// NewGoParser creates a new Go parser instance.
func NewGoParser() (*GoParser, error) {
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		return nil, fmt.Errorf("failed to create Go language: %w", err)
	}

	return &GoParser{
		supportedLanguage: goLang,
	}, nil
}

// ExtractModules extracts package information from a Go parse tree.
func (p *GoParser) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Go package", slogger.Fields{
		"include_metadata": options.IncludeMetadata,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	packageChunk := p.parseGoPackage(ctx, parseTree, options, time.Now())
	if packageChunk == nil {
		return []outbound.SemanticCodeChunk{}, nil
	}

	return []outbound.SemanticCodeChunk{*packageChunk}, nil
}

// GetSupportedLanguage returns the Go language instance.
func (p *GoParser) GetSupportedLanguage() valueobject.Language {
	return p.supportedLanguage
}

// GetSupportedConstructTypes returns the construct types supported by the Go parser.
func (p *GoParser) GetSupportedConstructTypes() []outbound.SemanticConstructType {
	return []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructMethod,
		outbound.ConstructStruct,
		outbound.ConstructInterface,
		outbound.ConstructVariable,
		outbound.ConstructConstant,
		outbound.ConstructPackage,
	}
}

// IsSupported checks if the given language is supported by this parser.
func (p *GoParser) IsSupported(language valueobject.Language) bool {
	return language.Name() == valueobject.LanguageGo
}

// ExtractMethodsFromStruct extracts methods associated with a specific struct.
func (p *GoParser) ExtractMethodsFromStruct(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	structName string,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Go methods for struct", slogger.Fields{
		"struct_name": structName,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	var methods []outbound.SemanticCodeChunk
	packageName := p.extractPackageNameFromTree(parseTree)
	now := time.Now()

	// Find method declarations with matching receiver
	methodNodes := parseTree.GetNodesByType("method_declaration")
	for _, node := range methodNodes {
		receiver := p.findChildByType(node, "parameter_list")
		if receiver != nil {
			receiverType := p.parseGoReceiver(parseTree, receiver)
			// Remove pointer prefix and check if it matches our struct
			baseType := strings.TrimPrefix(receiverType, "*")
			if baseType == structName {
				method := p.parseGoMethod(ctx, parseTree, node, packageName, options, now)
				if method != nil {
					methods = append(methods, *method)
				}
			}
		}
	}

	return methods, nil
}

// Helper methods for parsing Go constructs

// extractTypeDeclarations is a common helper for extracting struct and interface declarations.
func (p *GoParser) extractTypeDeclarations(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	targetType string,
	logMessage string,
	parseFunc func(context.Context, *valueobject.ParseTree, *valueobject.ParseNode, *valueobject.ParseNode, string, outbound.SemanticExtractionOptions, time.Time) *outbound.SemanticCodeChunk,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, logMessage, slogger.Fields{
		"include_private": options.IncludePrivate,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	var results []outbound.SemanticCodeChunk
	packageName := p.extractPackageNameFromTree(parseTree)
	now := time.Now()

	// Find type declarations
	typeNodes := parseTree.GetNodesByType("type_declaration")
	for _, node := range typeNodes {
		// Check if this is the target type
		typeSpec := p.findChildByType(node, "type_spec")
		if typeSpec == nil {
			continue
		}

		specificType := p.findChildByType(typeSpec, targetType)
		if specificType != nil {
			result := parseFunc(ctx, parseTree, node, typeSpec, packageName, options, now)
			if result != nil {
				results = append(results, *result)
			}
		}
	}

	return results, nil
}

// validateInput validates the input parse tree.
func (p *GoParser) validateInput(parseTree *valueobject.ParseTree) error {
	if parseTree == nil {
		return errors.New("parse tree cannot be nil")
	}

	if !p.IsSupported(parseTree.Language()) {
		return fmt.Errorf("unsupported language: %s, expected: %s",
			parseTree.Language().Name(), p.supportedLanguage.Name())
	}

	return nil
}

// extractPackageNameFromTree extracts package name from parse tree.
func (p *GoParser) extractPackageNameFromTree(parseTree *valueobject.ParseTree) string {
	packageNodes := parseTree.GetNodesByType("package_clause")
	if len(packageNodes) == 0 {
		return "main"
	}

	identifierNode := p.findChildByType(packageNodes[0], "package_identifier")
	if identifierNode == nil {
		return "main"
	}

	return parseTree.GetNodeText(identifierNode)
}

// findChildByType finds the first child node with the specified type.
func (p *GoParser) findChildByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	if node == nil {
		return nil
	}

	for _, child := range node.Children {
		if child.Type == nodeType {
			return child
		}
	}
	return nil
}

// findChildrenByType finds all child nodes with the specified type.
func (p *GoParser) findChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	if node == nil {
		return nil
	}

	var matches []*valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeType {
			matches = append(matches, child)
		}
	}
	return matches
}

// getVisibility determines visibility based on Go naming conventions.
func (p *GoParser) getVisibility(identifier string) outbound.VisibilityModifier {
	if len(identifier) > 0 && identifier[0] >= 'A' && identifier[0] <= 'Z' {
		return outbound.Public
	}
	return outbound.Private
}

// Removed unused helper functions - they were not being called

func (p *GoParser) parseGoPackage(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	packageName := p.extractPackageNameFromTree(parseTree)

	var documentation string
	if options.IncludeDocumentation {
		// Extract package-level comments
		documentation = p.extractPackageComments(parseTree)
	}

	var metadata map[string]interface{}
	if options.IncludeMetadata {
		metadata = p.extractPackageMetadata(parseTree)
	}

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("package", packageName, nil),
		Type:          outbound.ConstructPackage,
		Name:          packageName,
		QualifiedName: packageName,
		Language:      parseTree.Language(),
		StartByte:     0,
		EndByte:       utils.SafeUint32(len(parseTree.Source())),
		Content:       "package " + packageName,
		Documentation: documentation,
		Visibility:    outbound.Public,
		IsStatic:      true,
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash("package " + packageName),
	}
}

func (p *GoParser) extractPackageComments(_ *valueobject.ParseTree) string {
	// Minimal implementation - return empty for now
	return ""
}

func (p *GoParser) extractPackageMetadata(_ *valueobject.ParseTree) map[string]interface{} {
	// Minimal implementation - return empty for now
	return map[string]interface{}{}
}
