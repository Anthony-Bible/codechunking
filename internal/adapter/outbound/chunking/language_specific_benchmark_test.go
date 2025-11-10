package chunking

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"testing"
)

// BenchmarkGoLanguageSpecific benchmarks Go-specific chunking scenarios with complex struct hierarchies.
func BenchmarkGoLanguageSpecific(b *testing.B) {
	ctx := context.Background()
	_ = valueobject.Language{} // Explicit usage to prevent import removal

	// Create Go-specific code patterns
	goPatterns := []struct {
		name        string
		description string
		chunks      []outbound.SemanticCodeChunk
	}{
		{
			name:        "ComplexStructHierarchy",
			description: "Go structs with embedded types and interfaces",
			chunks:      generateGoStructHierarchy(),
		},
		{
			name:        "InterfaceImplementationPattern",
			description: "Multiple interface implementations",
			chunks:      generateGoInterfacePattern(),
		},
		{
			name:        "ConcurrentPatterns",
			description: "Go concurrency patterns with goroutines and channels",
			chunks:      generateGoConcurrentPatterns(),
		},
		{
			name:        "ErrorHandlingChains",
			description: "Complex Go error handling patterns",
			chunks:      generateGoErrorHandling(),
		},
	}

	benchmarks := []struct {
		name        string
		overlapSize int
		strategy    outbound.ChunkingStrategyType
	}{
		{"Function_NoOverlap", 0, outbound.StrategyFunction},
		{"Function_Overlap200", 200, outbound.StrategyFunction},
		{"Class_NoOverlap", 0, outbound.StrategyClass},
		{"Class_Overlap300", 300, outbound.StrategyClass},
		{"Hybrid_Overlap250", 250, outbound.StrategyHybrid},
	}

	for _, pattern := range goPatterns {
		b.Run(pattern.name, func(b *testing.B) {
			for _, bm := range benchmarks {
				b.Run(bm.name, func(b *testing.B) {
					b.ResetTimer()
					b.ReportAllocs()
					for range b.N {
						config := outbound.ChunkingConfiguration{
							Strategy:             bm.strategy,
							OverlapSize:          bm.overlapSize,
							IncludeDocumentation: true,
							EnableSplitting:      true,
							MaxChunkSize:         4000,
						}

						err := runChunkingStrategy(ctx, pattern.chunks, config, bm.strategy)
						if err != nil {
							b.Fatalf("Go pattern chunking failed: %v", err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkPythonLanguageSpecific benchmarks Python-specific chunking with class inheritance.
func BenchmarkPythonLanguageSpecific(b *testing.B) {
	ctx := context.Background()

	// Create Python-specific code patterns
	pythonPatterns := []struct {
		name        string
		description string
		chunks      []outbound.SemanticCodeChunk
	}{
		{
			name:        "MultipleInheritance",
			description: "Python classes with multiple inheritance",
			chunks:      generatePythonMultipleInheritance(),
		},
		{
			name:        "MetaclassPatterns",
			description: "Python metaclass usage patterns",
			chunks:      generatePythonMetaclassPattern(),
		},
		{
			name:        "DecoratorsAndGenerators",
			description: "Python decorators and generator functions",
			chunks:      generatePythonDecorators(),
		},
		{
			name:        "AsyncAwaitPatterns",
			description: "Python async/await patterns",
			chunks:      generatePythonAsyncPatterns(),
		},
	}

	benchmarks := []struct {
		name        string
		overlapSize int
		strategy    outbound.ChunkingStrategyType
	}{
		{"Function_NoOverlap", 0, outbound.StrategyFunction},
		{"Function_Overlap150", 150, outbound.StrategyFunction},
		{"Class_NoOverlap", 0, outbound.StrategyClass},
		{"Class_Overlap400", 400, outbound.StrategyClass},
		{"Hybrid_Overlap300", 300, outbound.StrategyHybrid},
	}

	for _, pattern := range pythonPatterns {
		b.Run(pattern.name, func(b *testing.B) {
			for _, bm := range benchmarks {
				b.Run(bm.name, func(b *testing.B) {
					b.ResetTimer()
					b.ReportAllocs()
					for range b.N {
						config := outbound.ChunkingConfiguration{
							Strategy:             bm.strategy,
							OverlapSize:          bm.overlapSize,
							IncludeDocumentation: true,
							EnableSplitting:      true,
							MaxChunkSize:         5000,
						}

						err := runChunkingStrategy(ctx, pattern.chunks, config, bm.strategy)
						if err != nil {
							b.Fatalf("Python pattern chunking failed: %v", err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkJavaScriptLanguageSpecific benchmarks JavaScript chunking with module dependencies.
func BenchmarkJavaScriptLanguageSpecific(b *testing.B) {
	ctx := context.Background()

	// Create JavaScript-specific code patterns
	jsPatterns := []struct {
		name        string
		description string
		chunks      []outbound.SemanticCodeChunk
	}{
		{
			name:        "ModuleDependencies",
			description: "JavaScript ES6 modules with imports/exports",
			chunks:      generateJSModulePattern(),
		},
		{
			name:        "PrototypePatterns",
			description: "JavaScript prototype-based inheritance",
			chunks:      generateJSPrototypePattern(),
		},
		{
			name:        "CallbackPromises",
			description: "JavaScript callbacks and promise chains",
			chunks:      generateJSPromisePattern(),
		},
		{
			name:        "ClosuresClosures",
			description: "JavaScript closure patterns",
			chunks:      generateJSClosurePattern(),
		},
	}

	benchmarks := []struct {
		name        string
		overlapSize int
		strategy    outbound.ChunkingStrategyType
	}{
		{"Function_NoOverlap", 0, outbound.StrategyFunction},
		{"Function_Overlap180", 180, outbound.StrategyFunction},
		{"Class_NoOverlap", 0, outbound.StrategyClass},
		{"Class_Overlap350", 350, outbound.StrategyClass},
		{"Hybrid_Overlap250", 250, outbound.StrategyHybrid},
	}

	for _, pattern := range jsPatterns {
		b.Run(pattern.name, func(b *testing.B) {
			for _, bm := range benchmarks {
				b.Run(bm.name, func(b *testing.B) {
					b.ResetTimer()
					b.ReportAllocs()
					for range b.N {
						config := outbound.ChunkingConfiguration{
							Strategy:             bm.strategy,
							OverlapSize:          bm.overlapSize,
							IncludeDocumentation: true,
							EnableSplitting:      true,
							MaxChunkSize:         4500,
						}

						err := runChunkingStrategy(ctx, pattern.chunks, config, bm.strategy)
						if err != nil {
							b.Fatalf("JavaScript pattern chunking failed: %v", err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkTypeScriptLanguageSpecific benchmarks TypeScript with type annotations and generics.
func BenchmarkTypeScriptLanguageSpecific(b *testing.B) {
	ctx := context.Background()

	// Create TypeScript-specific code patterns
	tsPatterns := []struct {
		name        string
		description string
		chunks      []outbound.SemanticCodeChunk
	}{
		{
			name:        "GenericTypes",
			description: "TypeScript generics and type constraints",
			chunks:      generateTSGenericsPattern(),
		},
		{
			name:        "UnionIntersectionTypes",
			description: "TypeScript union and intersection types",
			chunks:      generateTSUnionTypePattern(),
		},
		{
			name:        "DecoratorsMetadata",
			description: "TypeScript decorators and metadata",
			chunks:      generateTSDecoratorPattern(),
		},
		{
			name:        "NamespacesModules",
			description: "TypeScript namespaces and module patterns",
			chunks:      generateTSModulePattern(),
		},
	}

	benchmarks := []struct {
		name        string
		overlapSize int
		strategy    outbound.ChunkingStrategyType
	}{
		{"Function_NoOverlap", 0, outbound.StrategyFunction},
		{"Function_Overlap200", 200, outbound.StrategyFunction},
		{"Class_NoOverlap", 0, outbound.StrategyClass},
		{"Class_Overlap400", 400, outbound.StrategyClass},
		{"Hybrid_Overlap300", 300, outbound.StrategyHybrid},
	}

	for _, pattern := range tsPatterns {
		b.Run(pattern.name, func(b *testing.B) {
			for _, bm := range benchmarks {
				b.Run(bm.name, func(b *testing.B) {
					b.ResetTimer()
					b.ReportAllocs()
					for range b.N {
						config := outbound.ChunkingConfiguration{
							Strategy:             bm.strategy,
							OverlapSize:          bm.overlapSize,
							IncludeDocumentation: true,
							EnableSplitting:      true,
							MaxChunkSize:         5500,
						}

						err := runChunkingStrategy(ctx, pattern.chunks, config, bm.strategy)
						if err != nil {
							b.Fatalf("TypeScript pattern chunking failed: %v", err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkLanguageComparison compares performance across different languages.
func BenchmarkLanguageComparison(b *testing.B) {
	ctx := context.Background()

	// Equivalent functionality in different languages
	basePatterns := []struct {
		name     string
		language string
		chunks   []outbound.SemanticCodeChunk
	}{
		{"Go_Parser", "go", generateGoParserImplementation()},
		{"Python_Parser", "python", generatePythonParserImplementation()},
		{"TypeScript_Parser", "typescript", generateTypeScriptParserImplementation()},
	}

	benchmarks := []struct {
		name        string
		strategy    outbound.ChunkingStrategyType
		overlapSize int
	}{
		{"Strategy", outbound.StrategyFunction, 200},
		{"Strategy", outbound.StrategyClass, 300},
		{"Strategy", outbound.StrategyHybrid, 250},
	}

	for _, base := range basePatterns {
		for _, bm := range benchmarks {
			b.Run(fmt.Sprintf("%s_%s", base.name, bm.name), func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()
				for range b.N {
					config := outbound.ChunkingConfiguration{
						Strategy:        bm.strategy,
						OverlapSize:     bm.overlapSize,
						EnableSplitting: true,
						MaxChunkSize:    4000,
					}

					err := runChunkingStrategy(ctx, base.chunks, config, bm.strategy)
					if err != nil {
						b.Fatalf("Language comparison chunking failed: %v", err)
					}
				}
			})
		}
	}
}

// Helper functions for generating language-specific test data

func generateGoStructHierarchy() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Base interfaces and structs
	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:       "base-interface",
		Name:          "Processor",
		Signature:     "type Processor interface",
		Content:       generateGoBaseInterface(),
		Language:      mustCreateLanguage("go"),
		StartByte:     0,
		EndByte:       800,
		Type:          outbound.ConstructInterface,
		QualifiedName: "processor.Processor",
	})

	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:       "base-struct",
		Name:          "BaseProcessor",
		Signature:     "type BaseProcessor struct",
		Content:       generateGoBaseStruct(),
		Language:      mustCreateLanguage("go"),
		StartByte:     1000,
		EndByte:       1800,
		Type:          outbound.ConstructStruct,
		QualifiedName: "processor.BaseProcessor",
	})

	// Embedded struct
	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:       "embedded-struct",
		Name:          "DataProcessor",
		Signature:     "type DataProcessor struct",
		Content:       generateGoEmbeddedStruct(),
		Language:      mustCreateLanguage("go"),
		StartByte:     2000,
		EndByte:       3000,
		Type:          outbound.ConstructStruct,
		QualifiedName: "processor.DataProcessor",
	})

	// Methods
	for i := range 5 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("method-%d", i),
			Name:          fmt.Sprintf("Process%d", i),
			Signature:     fmt.Sprintf("func (p *DataProcessor) Process%d(data []byte) error", i),
			Content:       generateGoMethodContent(i),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(3000 + i*400),
			EndByte:       uint32(3400 + i*400),
			Type:          outbound.ConstructMethod,
			QualifiedName: fmt.Sprintf("processor.DataProcessor.Process%d", i),
		})
	}

	return chunks
}

func generateGoInterfacePattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Multiple interfaces
	interfaces := []string{"Reader", "Writer", "Closer"}
	for i, name := range interfaces {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("interface-%s", name),
			Name:          name,
			Signature:     fmt.Sprintf("type %s interface", name),
			Content:       generateGoInterfaceContent(name),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(i * 600),
			EndByte:       uint32((i + 1) * 600),
			Type:          outbound.ConstructInterface,
			QualifiedName: fmt.Sprintf("io.%s", name),
		})
	}

	// Implementation structs
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("impl-struct-%d", i),
			Name:          fmt.Sprintf("FileHandler%d", i),
			Signature:     fmt.Sprintf("type FileHandler%d struct", i),
			Content:       generateGoImplementationStruct(i),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(2000 + i*800),
			EndByte:       uint32(2400 + i*800),
			Type:          outbound.ConstructStruct,
			QualifiedName: fmt.Sprintf("handler.FileHandler%d", i),
		})
	}

	return chunks
}

func generateGoConcurrentPatterns() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Concurrent processing functions
	for i := range 4 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("concurrent-func-%d", i),
			Name:          fmt.Sprintf("ConcurrentProcess%d", i),
			Signature:     fmt.Sprintf("func ConcurrentProcess%d(data []byte) error", i),
			Content:       generateGoConcurrentFunction(i),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(i * 1200),
			EndByte:       uint32((i + 1) * 1200),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("concurrent.ConcurrentProcess%d", i),
		})
	}

	return chunks
}

func generateGoErrorHandling() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Error handling patterns
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("error-handler-%d", i),
			Name:          fmt.Sprintf("HandleError%d", i),
			Signature:     fmt.Sprintf("func HandleError%d(err error) error", i),
			Content:       generateGoErrorHandlingPattern(i),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(i * 1000),
			EndByte:       uint32((i + 1) * 1000),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("errors.HandleError%d", i),
		})
	}

	return chunks
}

func generatePythonMultipleInheritance() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Base classes
	baseClasses := []string{"Serializable", "Validatable", "Loggable"}
	for i, name := range baseClasses {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("base-class-%s", name),
			Name:          name,
			Signature:     fmt.Sprintf("class %s:", name),
			Content:       generatePythonBaseClass(name),
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(i * 400),
			EndByte:       uint32((i + 1) * 400),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("base.%s", name),
		})
	}

	// Multiple inheritance class
	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:       "multiple-inheritance",
		Name:          "DataProcessor",
		Signature:     "class DataProcessor(Serializable, Validatable, Loggable):",
		Content:       generatePythonMultipleInheritanceClass(),
		Language:      mustCreateLanguage("python"),
		StartByte:     1500,
		EndByte:       2500,
		Type:          outbound.ConstructClass,
		QualifiedName: "processor.DataProcessor",
	})

	return chunks
}

func generatePythonMetaclassPattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Metaclass
	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:       "metaclass",
		Name:          "SingletonMeta",
		Signature:     "class SingletonMeta(type):",
		Content:       generatePythonMetaclass(),
		Language:      mustCreateLanguage("python"),
		StartByte:     0,
		EndByte:       800,
		Type:          outbound.ConstructClass,
		QualifiedName: "patterns.SingletonMeta",
	})

	// Class using metaclass
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("singleton-class-%d", i),
			Name:          fmt.Sprintf("Service%d", i),
			Signature:     fmt.Sprintf("class Service%d(Singleton):", i),
			Content:       generatePythonSingletonClass(i),
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(1000 + i*600),
			EndByte:       uint32(1400 + i*600),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("services.Service%d", i),
		})
	}

	return chunks
}

func generatePythonDecorators() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Decorator functions
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("decorator-%d", i),
			Name:          fmt.Sprintf("decorator%d", i),
			Signature:     fmt.Sprintf("def decorator%d(func):", i),
			Content:       generatePythonDecorator(i),
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(i * 300),
			EndByte:       uint32((i + 1) * 300),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("decorators.decorator%d", i),
		})
	}

	// Classes and functions using decorators
	for i := range 4 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("decorated-class-%d", i),
			Name:          fmt.Sprintf("Processor%d", i),
			Signature:     fmt.Sprintf("@decorator%d\nclass Processor%d:", i%3, i),
			Content:       generatePythonDecoratedClass(i),
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(1200 + i*500),
			EndByte:       uint32(1500 + i*500),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("processors.Processor%d", i),
		})
	}

	return chunks
}

func generatePythonAsyncPatterns() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Async functions and generators
	for i := range 5 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("async-func-%d", i),
			Name:          fmt.Sprintf("asyncProcess%d", i),
			Signature:     fmt.Sprintf("async def asyncProcess%d(data: bytes) -> bytes:", i),
			Content:       generatePythonAsyncFunction(i),
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(i * 600),
			EndByte:       uint32((i + 1) * 600),
			Type:          outbound.ConstructAsyncFunction,
			QualifiedName: fmt.Sprintf("async.asyncProcess%d", i),
		})
	}

	return chunks
}

func generateJSModulePattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Module exports and imports
	for i := range 4 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("module-%d", i),
			Name:          fmt.Sprintf("module%d", i),
			Signature:     fmt.Sprintf("export function module%d(data)", i),
			Content:       generateJSModule(i),
			Language:      mustCreateLanguage("javascript"),
			StartByte:     uint32(i * 800),
			EndByte:       uint32((i + 1) * 800),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("modules.module%d", i),
		})
	}

	// Consumer module
	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:       "consumer",
		Name:          "consumer",
		Signature:     "import { module1, module2 } from './modules'",
		Content:       generateJSConsumer(),
		Language:      mustCreateLanguage("javascript"),
		StartByte:     4000,
		EndByte:       5000,
		Type:          outbound.ConstructModule,
		QualifiedName: "app.consumer",
	})

	return chunks
}

func generateJSPrototypePattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Prototype-based inheritance
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("prototype-class-%d", i),
			Name:          fmt.Sprintf("PrototypedClass%d", i),
			Signature:     fmt.Sprintf("function PrototypedClass%d(data)", i),
			Content:       generateJSPrototypeClass(i),
			Language:      mustCreateLanguage("javascript"),
			StartByte:     uint32(i * 1000),
			EndByte:       uint32((i + 1) * 1000),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("prototypes.PrototypedClass%d", i),
		})
	}

	return chunks
}

func generateJSPromisePattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Promise chains and async/await
	for i := range 4 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("promise-func-%d", i),
			Name:          fmt.Sprintf("promise%d", i),
			Signature:     fmt.Sprintf("function promise%d(data)", i),
			Content:       generateJSPromiseFunction(i),
			Language:      mustCreateLanguage("javascript"),
			StartByte:     uint32(i * 700),
			EndByte:       uint32((i + 1) * 700),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("promises.promise%d", i),
		})
	}

	return chunks
}

func generateJSClosurePattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Closure patterns
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("closure-func-%d", i),
			Name:          fmt.Sprintf("closure%d", i),
			Signature:     fmt.Sprintf("function closure%d(config)", i),
			Content:       generateJSClosureFunction(i),
			Language:      mustCreateLanguage("javascript"),
			StartByte:     uint32(i * 600),
			EndByte:       uint32((i + 1) * 600),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("closures.closure%d", i),
		})
	}

	return chunks
}

func generateTSGenericsPattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Generic classes and functions
	for i := range 4 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("generic-class-%d", i),
			Name:          fmt.Sprintf("GenericProcessor%d", i),
			Signature:     fmt.Sprintf("class GenericProcessor%d<T>", i),
			Content:       generateTSGenericClass(i),
			Language:      mustCreateLanguage("typescript"),
			StartByte:     uint32(i * 900),
			EndByte:       uint32((i + 1) * 900),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("generics.GenericProcessor%d", i),
		})
	}

	return chunks
}

func generateTSUnionTypePattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Union and intersection types
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("union-type-func-%d", i),
			Name:          fmt.Sprintf("processUnion%d", i),
			Signature:     fmt.Sprintf("function processUnion%d(value: string | number)", i),
			Content:       generateTSUnionTypeFunction(i),
			Language:      mustCreateLanguage("typescript"),
			StartByte:     uint32(i * 800),
			EndByte:       uint32((i + 1) * 800),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("uniontypes.processUnion%d", i),
		})
	}

	return chunks
}

func generateTSDecoratorPattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// TypeScript decorators
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("ts-decorator-%d", i),
			Name:          fmt.Sprintf("decorated%d", i),
			Signature:     fmt.Sprintf("@log\nclass Component%d", i),
			Content:       generateTSDecoratorClass(i),
			Language:      mustCreateLanguage("typescript"),
			StartByte:     uint32(i * 700),
			EndByte:       uint32((i + 1) * 700),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("decorators.Component%d", i),
		})
	}

	return chunks
}

func generateTSModulePattern() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// TypeScript namespaces and modules
	for i := range 3 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("ts-module-%d", i),
			Name:          fmt.Sprintf("Module%d", i),
			Signature:     fmt.Sprintf("namespace Module%d", i),
			Content:       generateTSModule(i),
			Language:      mustCreateLanguage("typescript"),
			StartByte:     uint32(i * 600),
			EndByte:       uint32((i + 1) * 600),
			Type:          outbound.ConstructNamespace,
			QualifiedName: fmt.Sprintf("modules.Module%d", i),
		})
	}

	return chunks
}

// Language-specific content generation functions.
func generateGoBaseInterface() string {
	return `type Processor interface {
    Process(data []byte) error
    Validate() error
    Close() error
}`
}

func generateGoBaseStruct() string {
	return `type BaseProcessor struct {
    mu     sync.RWMutex
    config Config
    logger *log.Logger
}

func (p *BaseProcessor) Validate() error {
    return nil
}`
}

func generateGoEmbeddedStruct() string {
	return `type DataProcessor struct {
    *BaseProcessor
    cache    map[string][]byte
    metrics  prometheus.Counter
    maxCache int
}

func (p *DataProcessor) Process(data []byte) error {
    return nil
}`
}

func generateGoMethodContent(index int) string {
	return fmt.Sprintf(`func (p *DataProcessor) Process%d(data []byte) error {
    batch := data[:min(1024, len(data))]
    if err := p.validateBatch%d(batch); err != nil {
        return err
    }
    return p.storeBatch%d(batch)
}`, index, index, index)
}

func generateGoInterfaceContent(name string) string {
	switch name {
	case "Reader":
		return `type Reader interface {
    Read() ([]byte, error)
    Close() error
}`
	case "Writer":
		return `type Writer interface {
    Write(data []byte) error
    Flush() error
}`
	case "Closer":
		return `type Closer interface {
    Close() error
}`
	default:
		return `type Unknown interface {}`
	}
}

func generateGoImplementationStruct(index int) string {
	return fmt.Sprintf(`type FileHandler%d struct {
    file *os.File
    name string
}

func (h *FileHandler%d) Read() ([]byte, error) {
    return nil, nil
}

func (h *FileHandler%d) Write(data []byte) error {
    return nil
}

func (h *FileHandler%d) Close() error {
    return nil
}`, index, index, index, index)
}

func generateGoConcurrentFunction(index int) string {
	return fmt.Sprintf(`func ConcurrentProcess%d(data []byte) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    var wg sync.WaitGroup
    errors := make(chan error, 10)

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            if err := processChunk%d(data[id*100:(id+1)*100]); err != nil {
                errors <- err
            }
        }(i)
    }

    wg.Wait()
    close(errors)
    return nil
}`, index, index)
}

func generateGoErrorHandlingPattern(index int) string {
	return fmt.Sprintf(`func HandleError%d(err error) error {
    if err == nil {
        return nil
    }

    var pathErr *os.PathError
    if errors.As(err, &pathErr) {
        return fmt.Errorf("file operation failed: %%w", pathErr)
    }

    if errors.Is(err, ErrNotFound) {
        return NewNotFoundError(err.Error())
    }

    return fmt.Errorf("unexpected error: %%w", err)
}`, index)
}

func generatePythonBaseClass(name string) string {
	template := `class %s:
    def __init__(self):
        self.data = {}
        self.timestamp = time.time()

    def to_dict(self):
        return {"data": self.data, "timestamp": self.timestamp}`
	return fmt.Sprintf(template, name)
}

func generatePythonMultipleInheritanceClass() string {
	return `class DataProcessor(Serializable, Validatable, Loggable):
    """Class with multiple inheritance."""

    def __init__(self):
        super().__init__()
        self.processed_count = 0

    def process(self, data):
        """Process data with all parent functionality."""
        self.validate(data)
        self.log_info(f"Processing {len(data)} items")
        result = self.transform(data)
        return self.to_dict(result)`
}

func generatePythonMetaclass() string {
	return `class SingletonMeta(type):
    _instances = {}

    def __call__(cls, *args, **kwargs):
        if cls not in cls._instances:
            instance = super().__call__(*args, **kwargs)
            cls._instances[cls] = instance
        return cls._instances[cls]

def Singleton(cls, bases, namespace):
    return SingletonMeta(cls.__name__, bases, namespace)`
}

func generatePythonSingletonClass(index int) string {
	return fmt.Sprintf(`class Service%d(Singleton):
    """Singleton service instance."""

    def __init__(self):
        self.service_id = %d
        self.initialized = True

    def process(self, data):
        return {"service_id": self.service_id, "data": data}`, index, index)
}

func generatePythonDecorator(index int) string {
	return fmt.Sprintf(`def decorator%d(func):
    def wrapper(*args, **kwargs):
        start = time.time()
        result = func(*args, **kwargs)
        duration = time.time() - start
        print(f"Function took {duration:.4f} seconds")
        return result
    return wrapper`, index)
}

func generatePythonDecoratedClass(index int) string {
	return fmt.Sprintf(`class Processor%d:
    """Decorated processor class."""

    @decorator%d
    def process(self, data):
        """Process data with timing decorator."""
        return [x.upper() for x in data]

    @decorator%d
    def validate(self, data):
        """Validate data with timing decorator."""
        return len(data) > 0`, index, index, index)
}

func generatePythonAsyncFunction(index int) string {
	return fmt.Sprintf(`async def asyncProcess%d(data: bytes) -> bytes:
    """Async processing function."""
    async with aiofiles.open(f"input_{%d}.txt", "rb") as f:
        content = await f.read()

    processed = content + data

    async with aiofiles.open(f"output_{%d}.txt", "wb") as f:
        await f.write(processed)

    return processed`, index, index, index)
}

func generateJSModule(index int) string {
	return fmt.Sprintf(`// Module %d functionality
import { Logger } from './logger';
import { Validator } from './validator';

export function module%d(data) {
    Logger.info("Processing data " + data.length);
    if (!Validator.isValid(data)) {
        throw new Error('Invalid data');
    }
    return data.map(item => item.toUpperCase());
}

export default module%d;`, index, index, index)
}

func generateJSConsumer() string {
	return `// Consumer module
import { module1, module2 } from './modules';
import { processResults } from './utils';

class Consumer {
    constructor() {
        this.processors = [module1, module2];
    }

    handle(data) {
        const results = this.processors.map(processor => processor(data));
        return processResults(results);
    }
}`
}

func generateJSPrototypeClass(index int) string {
	return fmt.Sprintf(`function PrototypedClass%d(data) {
    this.data = data;
    this.id = Date.now();
}

PrototypedClass%d.prototype.process = function() {
    return this.data.map(item => item.value * 2);
};

PrototypedClass%d.prototype.validate = function() {
    return Array.isArray(this.data);
};`, index, index, index)
}

func generateJSPromiseFunction(index int) string {
	return fmt.Sprintf(`function promise%d(data) {
    return new Promise((resolve, reject) => {
        setTimeout(() => {
            if (data && data.length > 0) {
                resolve(data.filter(item => item.active));
            } else {
                reject(new Error('Invalid data'));
            }
        }, %d);
    })
    .then(result => result.map(item => ({ ...item, processed: true })))
    .catch(error => ({ error: error.message }));
}`, index, (index+1)*100)
}

func generateJSClosureFunction(index int) string {
	return fmt.Sprintf(`function closure%d(config) {
    let counter = 0;

    return {
        increment: function() {
            return ++counter;
        },
        getCounter: function() {
            return counter + config.offset || 0;
        },
        reset: function() {
            counter = 0;
        }
    };
}`, index)
}

func generateTSGenericClass(index int) string {
	return fmt.Sprintf(`class GenericProcessor%d<T> {
    private cache: Map<string, T> = new Map();

    constructor(private processor: (item: T) => T) {}

    process(items: T[]): T[] {
        return items.map(item => this.getItemOrProcess(item));
    }

    private getItemOrProcess(item: T): T {
        const key = JSON.stringify(item);
        if (!this.cache.has(key)) {
            this.cache.set(key, this.processor(item));
        }
        return this.cache.get(key)!;
    }
}`, index)
}

func generateTSUnionTypeFunction(index int) string {
	return fmt.Sprintf(`function processUnion%d(value: string | number): string {
    if (typeof value === 'string') {
        return value.toUpperCase();
    } else if (typeof value === 'number') {
        return value.toFixed(%d);
    }
    throw new Error('Unexpected type');
}`, index, index%2)
}

func generateTSDecoratorClass(index int) string {
	return fmt.Sprintf(`function log%d(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    const originalMethod = descriptor.value;
    descriptor.value = function(...args: any[]) {
        console.log("Called " + propertyKey + " with args:", args);
        const result = originalMethod.apply(this, args);
        console.log(propertyKey + " returned:", result);
        return result;
    };
}

@log%d
class Component%d {
    private id: string;

    constructor(id: string) {
        this.id = id;
    }

    render(): string {
        return "<div id=\"" + this.id + "\">Component %d</div>";
    }
}`, index, index, index, index)
}

func generateTSModule(index int) string {
	return fmt.Sprintf(`namespace Module%d {
    export interface Config {
        enabled: boolean;
        version: string;
    }

    export class Manager {
        private config: Config;

        constructor(config: Config) {
            this.config = config;
        }

        isEnabled(): boolean {
            return this.config.enabled;
        }
    }

    export const defaultConfig: Config = {
        enabled: true,
        version: "1.0.0"
    };
}`, index)
}

func generateGoParserImplementation() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Parser interfaces and implementations
	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:   "parser-interface",
		Name:      "Parser",
		Signature: "type Parser interface",
		Content: `type Parser interface {
    Parse(input string) (AST, error)
    Validate(ast AST) error
}`,
		Language:      mustCreateLanguage("go"),
		StartByte:     0,
		EndByte:       200,
		Type:          outbound.ConstructInterface,
		QualifiedName: "parser.Parser",
	})

	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:   "go-parser",
		Name:      "GoParser",
		Signature: "type GoParser struct",
		Content: `type GoParser struct {
    lexer  *Lexer
    tokens []Token
    index  int
}

func (p *GoParser) Parse(input string) (AST, error) {
    p.lexer = NewLexer(input)
    p.tokens = p.lexer.Tokenize()
    return p.buildAST()
}`,
		Language:      mustCreateLanguage("go"),
		StartByte:     300,
		EndByte:       800,
		Type:          outbound.ConstructStruct,
		QualifiedName: "parser.GoParser",
	})

	return chunks
}

func generatePythonParserImplementation() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:   "python-parser",
		Name:      "PythonParser",
		Signature: "class PythonParser:",
		Content: `class PythonParser:
    """Python AST parser implementation."""

    def __init__(self):
        self.tokens = []
        self.current = 0

    def parse(self, source: str) -> AST:
        lex = Lexer(source)
        self.tokens = lex.tokenize()
        return self.build_ast()`,
		Language:      mustCreateLanguage("python"),
		StartByte:     0,
		EndByte:       600,
		Type:          outbound.ConstructClass,
		QualifiedName: "parser.PythonParser",
	})

	return chunks
}

func generateTypeScriptParserImplementation() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	chunks = append(chunks, outbound.SemanticCodeChunk{
		ChunkID:   "ts-parser",
		Name:      "TypeScriptParser",
		Signature: "class TypeScriptParser {",
		Content: `class TypeScriptParser {
    private tokens: Token[] = [];
    private index: number = 0;

    constructor(lexer: Lexer) {
        this.lexer = lexer;
    }

    parse(source: string): AST {
        this.tokens = this.lexer.tokenize(source);
        return this.buildAST();
    }
}`,
		Language:      mustCreateLanguage("typescript"),
		StartByte:     0,
		EndByte:       700,
		Type:          outbound.ConstructClass,
		QualifiedName: "parser.TypeScriptParser",
	})

	return chunks
}
