package chunking

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
)

// ClassRelationshipAnalyzer handles analysis of class relationships and hierarchies
// for intelligent class-level chunking with optimized performance.
type ClassRelationshipAnalyzer struct{}

// NewClassRelationshipAnalyzer creates a new class relationship analyzer.
func NewClassRelationshipAnalyzer() *ClassRelationshipAnalyzer {
	return &ClassRelationshipAnalyzer{}
}

// EnhancedClassInfo extends the basic ClassInfo with relationship analysis capabilities.
type EnhancedClassInfo struct {
	*ClassInfo

	Relationships []ClassRelationship
}

// ClassRelationship represents a relationship between classes.
type ClassRelationship struct {
	Type        RelationshipType
	TargetClass string
	Strength    float64
	IsResolved  bool
	Metadata    map[string]interface{}
}

// RelationshipType defines types of class relationships.
type RelationshipType string

const (
	RelationshipInheritance RelationshipType = "inheritance"
	RelationshipImplements  RelationshipType = "implements"
	RelationshipComposition RelationshipType = "composition"
	RelationshipAggregation RelationshipType = "aggregation"
	RelationshipNested      RelationshipType = "nested"
	RelationshipDependency  RelationshipType = "dependency"
)

// AnalyzeClassRelationships performs comprehensive analysis of class relationships
// with performance optimizations for large codebases.
func (c *ClassRelationshipAnalyzer) AnalyzeClassRelationships(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
) (map[string]*EnhancedClassInfo, error) {
	slogger.Debug(ctx, "Starting class relationship analysis", slogger.Fields{
		"chunk_count": len(semanticChunks),
	})

	// Pre-allocate maps with estimated capacity for better performance
	classMap := make(map[string]*EnhancedClassInfo, len(semanticChunks)/4)
	methodMap := make(map[string][]outbound.SemanticCodeChunk, len(semanticChunks)/2)
	propertyMap := make(map[string][]outbound.SemanticCodeChunk, len(semanticChunks)/2)

	// First pass: identify all class-like constructs
	c.identifyClasses(ctx, semanticChunks, classMap)

	// Second pass: group methods and properties by class
	c.groupMethodsAndProperties(ctx, semanticChunks, methodMap, propertyMap)

	// Third pass: associate methods and properties with classes
	c.associateClassMembers(ctx, classMap, methodMap, propertyMap)

	// Fourth pass: analyze inheritance and implementation relationships
	c.analyzeInheritanceRelationships(ctx, classMap)

	// Fifth pass: handle nested classes and composition
	c.analyzeCompositionAndNesting(ctx, classMap)

	slogger.Debug(ctx, "Class relationship analysis completed", slogger.Fields{
		"classes_found":      len(classMap),
		"methods_grouped":    len(methodMap),
		"properties_grouped": len(propertyMap),
	})

	return classMap, nil
}

// identifyClasses identifies all class-like constructs and creates EnhancedClassInfo entries.
func (c *ClassRelationshipAnalyzer) identifyClasses(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	classMap map[string]*EnhancedClassInfo,
) {
	for _, chunk := range semanticChunks {
		if !c.isClassLikeConstruct(chunk.Type) {
			continue
		}

		basicClassInfo := &ClassInfo{
			Name:          chunk.Name,
			QualifiedName: chunk.QualifiedName,
			Chunk:         chunk,
			Methods:       make([]outbound.SemanticCodeChunk, 0, 10),
			Properties:    make([]outbound.SemanticCodeChunk, 0, 5),
			Language:      chunk.Language,
		}

		enhancedClassInfo := &EnhancedClassInfo{
			ClassInfo:     basicClassInfo,
			Relationships: make([]ClassRelationship, 0, 3),
		}

		classMap[chunk.QualifiedName] = enhancedClassInfo
	}
}

// groupMethodsAndProperties groups methods and properties by their parent class names.
func (c *ClassRelationshipAnalyzer) groupMethodsAndProperties(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	methodMap map[string][]outbound.SemanticCodeChunk,
	propertyMap map[string][]outbound.SemanticCodeChunk,
) {
	for _, chunk := range semanticChunks {
		switch chunk.Type {
		case outbound.ConstructMethod, outbound.ConstructFunction:
			parentClass := c.extractParentClassName(chunk)
			if parentClass != "" {
				methodMap[parentClass] = append(methodMap[parentClass], chunk)
			}
		case outbound.ConstructField, outbound.ConstructProperty:
			parentClass := c.extractParentClassName(chunk)
			if parentClass != "" {
				propertyMap[parentClass] = append(propertyMap[parentClass], chunk)
			}
		case outbound.ConstructClass, outbound.ConstructStruct, outbound.ConstructInterface,
			outbound.ConstructEnum, outbound.ConstructVariable, outbound.ConstructConstant,
			outbound.ConstructModule, outbound.ConstructPackage, outbound.ConstructNamespace,
			outbound.ConstructType, outbound.ConstructComment, outbound.ConstructDecorator,
			outbound.ConstructAttribute, outbound.ConstructLambda, outbound.ConstructClosure,
			outbound.ConstructGenerator, outbound.ConstructAsyncFunction:
			// These types are handled in other passes or not relevant for method/property grouping
		}
	}
}

// associateClassMembers associates methods and properties with their parent classes.
func (c *ClassRelationshipAnalyzer) associateClassMembers(
	ctx context.Context,
	classMap map[string]*EnhancedClassInfo,
	methodMap map[string][]outbound.SemanticCodeChunk,
	propertyMap map[string][]outbound.SemanticCodeChunk,
) {
	for qualifiedName, classInfo := range classMap {
		// Associate methods
		if methods, exists := methodMap[classInfo.Name]; exists {
			classInfo.Methods = append(classInfo.Methods, methods...)
		}
		if methods, exists := methodMap[qualifiedName]; exists {
			classInfo.Methods = append(classInfo.Methods, methods...)
		}

		// Associate properties
		if properties, exists := propertyMap[classInfo.Name]; exists {
			classInfo.Properties = append(classInfo.Properties, properties...)
		}
		if properties, exists := propertyMap[qualifiedName]; exists {
			classInfo.Properties = append(classInfo.Properties, properties...)
		}

		// Also try to find members by byte range containment
		c.associateMembersByByteRange(classInfo.ClassInfo, methodMap, propertyMap)
	}
}

// associateMembersByByteRange associates members that fall within the class's byte range.
func (c *ClassRelationshipAnalyzer) associateMembersByByteRange(
	classInfo *ClassInfo,
	methodMap map[string][]outbound.SemanticCodeChunk,
	propertyMap map[string][]outbound.SemanticCodeChunk,
) {
	// Check all methods for byte range containment
	for _, methods := range methodMap {
		for _, method := range methods {
			if c.isChunkContainedInClass(method, *classInfo) {
				// Check if not already associated
				if !c.isMethodAlreadyAssociated(method, classInfo.Methods) {
					classInfo.Methods = append(classInfo.Methods, method)
				}
			}
		}
	}

	// Check all properties for byte range containment
	for _, properties := range propertyMap {
		for _, property := range properties {
			if c.isChunkContainedInClass(property, *classInfo) {
				// Check if not already associated
				if !c.isPropertyAlreadyAssociated(property, classInfo.Properties) {
					classInfo.Properties = append(classInfo.Properties, property)
				}
			}
		}
	}
}

// analyzeInheritanceRelationships analyzes inheritance and interface implementation relationships.
func (c *ClassRelationshipAnalyzer) analyzeInheritanceRelationships(
	ctx context.Context,
	classMap map[string]*EnhancedClassInfo,
) {
	for _, classInfo := range classMap {
		// Analyze explicit dependencies from the chunk
		for _, dep := range classInfo.Chunk.Dependencies {
			relationship := c.determineRelationshipType(dep, classInfo.Chunk)
			classInfo.Relationships = append(classInfo.Relationships, ClassRelationship{
				Type:        relationship,
				TargetClass: dep.Name,
				Strength:    c.calculateRelationshipStrength(relationship),
				IsResolved:  c.isRelationshipResolved(dep.Name, classMap),
				Metadata:    map[string]interface{}{"source": "explicit_dependency"},
			})
		}

		// Analyze type usage patterns for implicit relationships
		for _, typeRef := range classInfo.Chunk.UsedTypes {
			if c.isClassType(typeRef.Name, classMap) {
				classInfo.Relationships = append(classInfo.Relationships, ClassRelationship{
					Type:        RelationshipDependency,
					TargetClass: typeRef.Name,
					Strength:    0.6,
					IsResolved:  true,
					Metadata:    map[string]interface{}{"source": "type_usage"},
				})
			}
		}
	}
}

// analyzeCompositionAndNesting analyzes composition and nested class relationships.
func (c *ClassRelationshipAnalyzer) analyzeCompositionAndNesting(
	ctx context.Context,
	classMap map[string]*EnhancedClassInfo,
) {
	// Find nested classes based on byte ranges
	for _, classInfo := range classMap {
		for _, otherClass := range classMap {
			if classInfo.QualifiedName == otherClass.QualifiedName {
				continue
			}

			// Check for nesting relationship
			if c.isClassNestedInClass(*classInfo.ClassInfo, *otherClass.ClassInfo) {
				otherClass.Relationships = append(otherClass.Relationships, ClassRelationship{
					Type:        RelationshipNested,
					TargetClass: classInfo.QualifiedName,
					Strength:    0.95,
					IsResolved:  true,
					Metadata:    map[string]interface{}{"nesting_type": "byte_range"},
				})
			}
		}
	}

	// Merge nested classes into their parents
	c.mergeNestedClasses(ctx, classMap)
}

// Helper methods

func (c *ClassRelationshipAnalyzer) isClassLikeConstruct(constructType outbound.SemanticConstructType) bool {
	switch constructType {
	case outbound.ConstructClass, outbound.ConstructStruct, outbound.ConstructInterface:
		return true
	case outbound.ConstructFunction, outbound.ConstructMethod, outbound.ConstructEnum,
		outbound.ConstructVariable, outbound.ConstructConstant, outbound.ConstructField,
		outbound.ConstructProperty, outbound.ConstructModule, outbound.ConstructPackage,
		outbound.ConstructNamespace, outbound.ConstructType, outbound.ConstructComment,
		outbound.ConstructDecorator, outbound.ConstructAttribute, outbound.ConstructLambda,
		outbound.ConstructClosure, outbound.ConstructGenerator, outbound.ConstructAsyncFunction:
		return false
	default:
		return false
	}
}

func (c *ClassRelationshipAnalyzer) extractParentClassName(chunk outbound.SemanticCodeChunk) string {
	// For methods with qualified names like "ClassName.MethodName"
	if strings.Contains(chunk.QualifiedName, ".") {
		parts := strings.Split(chunk.QualifiedName, ".")
		if len(parts) >= 2 {
			// Handle receiver types like "*ClassName.MethodName"
			parentName := parts[len(parts)-2]
			parentName = strings.TrimPrefix(parentName, "*")
			return parentName
		}
	}

	return ""
}

func (c *ClassRelationshipAnalyzer) isChunkContainedInClass(
	chunk outbound.SemanticCodeChunk,
	classInfo ClassInfo,
) bool {
	return chunk.StartByte >= classInfo.Chunk.StartByte &&
		chunk.EndByte <= classInfo.Chunk.EndByte
}

func (c *ClassRelationshipAnalyzer) isMethodAlreadyAssociated(
	method outbound.SemanticCodeChunk,
	methods []outbound.SemanticCodeChunk,
) bool {
	for _, existingMethod := range methods {
		if existingMethod.ID == method.ID {
			return true
		}
	}
	return false
}

func (c *ClassRelationshipAnalyzer) isPropertyAlreadyAssociated(
	property outbound.SemanticCodeChunk,
	properties []outbound.SemanticCodeChunk,
) bool {
	for _, existingProperty := range properties {
		if existingProperty.ID == property.ID {
			return true
		}
	}
	return false
}

func (c *ClassRelationshipAnalyzer) determineRelationshipType(
	dep outbound.DependencyReference,
	chunk outbound.SemanticCodeChunk,
) RelationshipType {
	switch strings.ToLower(dep.Type) {
	case "inheritance", "extends", "inherits":
		return RelationshipInheritance
	case "interface", "implements":
		return RelationshipImplements
	case "composition", "has-a":
		return RelationshipComposition
	case "aggregation", "uses":
		return RelationshipAggregation
	default:
		return RelationshipDependency
	}
}

func (c *ClassRelationshipAnalyzer) calculateRelationshipStrength(relType RelationshipType) float64 {
	switch relType {
	case RelationshipInheritance:
		return 0.95
	case RelationshipImplements:
		return 0.90
	case RelationshipNested:
		return 0.95
	case RelationshipComposition:
		return 0.85
	case RelationshipAggregation:
		return 0.75
	case RelationshipDependency:
		return 0.60
	default:
		return 0.50
	}
}

func (c *ClassRelationshipAnalyzer) isRelationshipResolved(
	targetClass string,
	classMap map[string]*EnhancedClassInfo,
) bool {
	_, exists := classMap[targetClass]
	return exists
}

func (c *ClassRelationshipAnalyzer) isClassType(
	typeName string,
	classMap map[string]*EnhancedClassInfo,
) bool {
	_, exists := classMap[typeName]
	return exists
}

func (c *ClassRelationshipAnalyzer) isClassNestedInClass(
	innerClass ClassInfo,
	outerClass ClassInfo,
) bool {
	// Check byte range containment
	return innerClass.Chunk.StartByte >= outerClass.Chunk.StartByte &&
		innerClass.Chunk.EndByte <= outerClass.Chunk.EndByte &&
		innerClass.QualifiedName != outerClass.QualifiedName
}

func (c *ClassRelationshipAnalyzer) mergeNestedClasses(
	ctx context.Context,
	classMap map[string]*EnhancedClassInfo,
) {
	var toRemove []string

	for _, classInfo := range classMap {
		for _, relationship := range classInfo.Relationships {
			if relationship.Type == RelationshipNested {
				// Find the nested class and merge its content into the parent
				if nestedClass, exists := classMap[relationship.TargetClass]; exists {
					classInfo.Methods = append(classInfo.Methods, nestedClass.Methods...)
					classInfo.Properties = append(classInfo.Properties, nestedClass.Properties...)

					// Mark nested class for removal
					toRemove = append(toRemove, relationship.TargetClass)
				}
			}
		}
	}

	// Remove nested classes from the main map
	for _, qualifiedName := range toRemove {
		delete(classMap, qualifiedName)
	}

	slogger.Debug(ctx, "Merged nested classes", slogger.Fields{
		"nested_classes_merged": len(toRemove),
	})
}
