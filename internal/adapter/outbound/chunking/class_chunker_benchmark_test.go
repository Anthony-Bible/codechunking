package chunking

import (
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"testing"
)

// BenchmarkClassChunkingWithOverlap benchmarks class-level chunking with various overlap sizes.
func BenchmarkClassChunkingWithOverlap(b *testing.B) {
	chunker := NewClassChunker()
	ctx := context.Background()

	// Create test semantic chunks for a complex class system
	goClass := outbound.SemanticCodeChunk{
		ChunkID:       "go-struct",
		Name:          "UserService",
		Signature:     "type UserService struct",
		Content:       createGoClassContent(),
		Language:      mustCreateLanguage("go"),
		StartByte:     0,
		EndByte:       800,
		Type:          outbound.ConstructStruct,
		QualifiedName: "app.UserService",
	}

	pythonClass := outbound.SemanticCodeChunk{
		ChunkID:       "python-class",
		Name:          "DataProcessor",
		Signature:     "class DataProcessor:",
		Content:       createPythonClassContent(),
		Language:      mustCreateLanguage("python"),
		StartByte:     1000,
		EndByte:       1200,
		Type:          outbound.ConstructClass,
		QualifiedName: "models.DataProcessor",
	}

	typescriptInterface := outbound.SemanticCodeChunk{
		ChunkID:       "ts-interface",
		Name:          "IHttpResponse",
		Signature:     "interface IHttpResponse",
		Content:       createTypeScriptInterfaceContent(),
		Language:      mustCreateLanguage("typescript"),
		StartByte:     1300,
		EndByte:       1600,
		Type:          outbound.ConstructInterface,
		QualifiedName: "http.IHttpResponse",
	}

	// Add methods and properties
	goMethods := createGoMethodChunks(goClass.QualifiedName)
	pythonMethods := createPythonMethodChunks(pythonClass.QualifiedName)
	tsMethods := createTypeScriptMethodChunks(typescriptInterface.QualifiedName)

	// Combine all chunks
	allChunks := []outbound.SemanticCodeChunk{
		goClass, pythonClass, typescriptInterface,
	}
	allChunks = append(allChunks, goMethods...)
	allChunks = append(allChunks, pythonMethods...)
	allChunks = append(allChunks, tsMethods...)

	benchmarks := []struct {
		name        string
		overlapSize int
	}{
		{"NoOverlap", 0},
		{"Overlap100", 100},
		{"Overlap200", 200},
		{"Overlap500", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:             outbound.StrategyClass,
					OverlapSize:          bm.overlapSize,
					IncludeDocumentation: true,
					EnableSplitting:      true,
				}

				_, err := chunker.ChunkByClass(ctx, allChunks, config)
				if err != nil {
					b.Fatalf("Class chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkClassChunkingMemoryAllocation benchmarks memory allocation patterns in class chunking.
func BenchmarkClassChunkingMemoryAllocation(b *testing.B) {
	chunker := NewClassChunker()
	ctx := context.Background()

	// Create a large codebase with many classes
	classes := make([]outbound.SemanticCodeChunk, 20)
	for i := range 20 {
		content := createLargeClassContent(i)
		classes[i] = outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("class-%d", i),
			Name:          fmt.Sprintf("Class%d", i),
			Signature:     fmt.Sprintf("class Class%d:", i),
			Content:       content,
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(i * 2000),
			EndByte:       uint32((i + 1) * 2000),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("models.Class%d", i),
		}
	}

	// Add methods to classes
	var methods []outbound.SemanticCodeChunk
	for i := range 20 {
		for j := range 5 {
			method := outbound.SemanticCodeChunk{
				ChunkID:       fmt.Sprintf("method-%d-%d", i, j),
				Name:          fmt.Sprintf("method%d", j),
				Signature:     fmt.Sprintf("def method%d(self, param):", j),
				Content:       createMethodContent(i, j),
				Language:      mustCreateLanguage("python"),
				StartByte:     uint32(i*2000 + 1500 + j*100),
				EndByte:       uint32(i*2000 + 1500 + (j+1)*100),
				Type:          outbound.ConstructMethod,
				QualifiedName: fmt.Sprintf("models.Class%d.method%d", i, j),
			}
			methods = append(methods, method)
		}
	}

	allChunks := classes
	allChunks = append(allChunks, methods...)

	benchmarks := []struct {
		name        string
		overlapSize int
	}{
		{"NoOverlap", 0},
		{"SmallOverlap", 100},
		{"LargeOverlap", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategyClass,
					OverlapSize:     bm.overlapSize,
					EnableSplitting: true,
					MaxChunkSize:    3000,
				}

				_, err := chunker.ChunkByClass(ctx, allChunks, config)
				if err != nil {
					b.Fatalf("Class chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkClassChunkingWithInheritance tests performance with inheritance hierarchies.
func BenchmarkClassChunkingWithInheritance(b *testing.B) {
	chunker := NewClassChunker()
	ctx := context.Background()

	// Create inheritance hierarchy
	baseClass := outbound.SemanticCodeChunk{
		ChunkID:       "base-class",
		Name:          "BaseService",
		Signature:     "class BaseService:",
		Content:       createBaseClassContent(),
		Language:      mustCreateLanguage("python"),
		StartByte:     0,
		EndByte:       400,
		Type:          outbound.ConstructClass,
		QualifiedName: "services.BaseService",
		Dependencies: []outbound.DependencyReference{
			{Name: "ABC"},
			{Name: "LoggerInterface"},
		},
	}

	// Create derived classes
	derivedClasses := make([]outbound.SemanticCodeChunk, 5)
	for i := range 5 {
		derivedClasses[i] = outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("derived-class-%d", i),
			Name:          fmt.Sprintf("Service%d", i),
			Signature:     fmt.Sprintf("class Service%d(BaseService):", i),
			Content:       createDerivedClassContent(i),
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(500 + i*800),
			EndByte:       uint32(1000 + i*800),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("services.Service%d", i),
			Dependencies: []outbound.DependencyReference{
				{Name: "BaseService"},
				{Name: fmt.Sprintf("Repository%d", i)},
			},
		}
	}

	allChunks := append([]outbound.SemanticCodeChunk{baseClass}, derivedClasses...)

	benchmarks := []struct {
		name            string
		overlapSize     int
		enableSplitting bool
		includeDeps     bool
	}{
		{"NoOverlapNoSplit", 0, false, false},
		{"WithOverlap", 200, false, false},
		{"WithSplitting", 200, true, false},
		{"FullFeatures", 300, true, true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategyClass,
					OverlapSize:     bm.overlapSize,
					EnableSplitting: bm.enableSplitting,
					MaxChunkSize:    1000,
					MinChunkSize:    200,
				}

				_, err := chunker.ChunkByClass(ctx, allChunks, config)
				if err != nil {
					b.Fatalf("Class chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkClassSplittingPerformance benchmarks performance of large class splitting.
func BenchmarkClassSplittingPerformance(b *testing.B) {
	chunker := NewClassChunker()
	ctx := context.Background()

	sizes := []int{
		1, 5, 10, 25, 50, 100,
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Classes%d", size), func(b *testing.B) {
			// Create large classes that exceed max chunk size
			classes := make([]outbound.SemanticCodeChunk, size)
			for i := range size {
				classes[i] = outbound.SemanticCodeChunk{
					ChunkID:       fmt.Sprintf("large-class-%d", i),
					Name:          fmt.Sprintf("LargeClass%d", i),
					Signature:     fmt.Sprintf("class LargeClass%d:", i),
					Content:       createVeryLargeClassContent(i),
					Language:      mustCreateLanguage("python"),
					StartByte:     uint32(i * 5000),
					EndByte:       uint32((i + 1) * 5000),
					Type:          outbound.ConstructClass,
					QualifiedName: fmt.Sprintf("models.LargeClass%d", i),
				}
			}

			// Add many methods to each class
			allChunks := make([]outbound.SemanticCodeChunk, 0, len(classes))
			allChunks = append(allChunks, classes...)

			for i := range classes {
				for j := range 20 {
					method := outbound.SemanticCodeChunk{
						ChunkID:       fmt.Sprintf("method-%d-%d", i, j),
						Name:          fmt.Sprintf("process%d", j),
						Signature:     fmt.Sprintf("def process%d(self, data):", j),
						Content:       createComplexMethodContent(j),
						Language:      mustCreateLanguage("python"),
						StartByte:     uint32(i*5000 + 2000 + j*150),
						EndByte:       uint32(i*5000 + 2000 + (j+1)*150),
						Type:          outbound.ConstructMethod,
						QualifiedName: fmt.Sprintf("models.LargeClass%d.process%d", i, j),
					}
					allChunks = append(allChunks, method)
				}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategyClass,
					OverlapSize:     200,
					EnableSplitting: true,
					MaxChunkSize:    2000,
					MinChunkSize:    500,
				}

				_, err := chunker.ChunkByClass(ctx, allChunks, config)
				if err != nil {
					b.Fatalf("Class chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkNestedClassChunking tests performance with nested classes.
func BenchmarkNestedClassChunking(b *testing.B) {
	chunker := NewClassChunker()
	ctx := context.Background()

	// Create nested class hierarchy
	mainClass := outbound.SemanticCodeChunk{
		ChunkID:       "main-class",
		Name:          "Container",
		Signature:     "class Container:",
		Content:       createContainerClassContent(),
		Language:      mustCreateLanguage("python"),
		StartByte:     0,
		EndByte:       800,
		Type:          outbound.ConstructClass,
		QualifiedName: "models.Container",
	}

	// Create nested classes
	nestedClasses := make([]outbound.SemanticCodeChunk, 10)
	for i := range 10 {
		nestedClasses[i] = outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("nested-class-%d", i),
			Name:          fmt.Sprintf("InnerClass%d", i),
			Signature:     fmt.Sprintf("    class InnerClass%d:", i),
			Content:       createNestedClassContent(i),
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(100 + i*60),
			EndByte:       uint32(150 + i*60),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("models.Container.InnerClass%d", i),
		}
	}

	// Add nested methods
	nestedMethods := make([]outbound.SemanticCodeChunk, 0)
	for i := range 10 {
		for j := range 3 {
			method := outbound.SemanticCodeChunk{
				ChunkID:       fmt.Sprintf("nested-method-%d-%d", i, j),
				Name:          fmt.Sprintf("nested_method%d", j),
				Signature:     fmt.Sprintf("        def nested_method%d(self, param):", j),
				Content:       createNestedMethodContent(j),
				Language:      mustCreateLanguage("python"),
				StartByte:     uint32(150 + i*60 + j*50),
				EndByte:       uint32(180 + i*60 + j*50),
				Type:          outbound.ConstructMethod,
				QualifiedName: fmt.Sprintf("models.Container.InnerClass%d.nested_method%d", i, j),
			}
			nestedMethods = append(nestedMethods, method)
		}
	}

	allChunks := append([]outbound.SemanticCodeChunk{mainClass}, nestedClasses...)
	allChunks = append(allChunks, nestedMethods...)

	benchmarks := []struct {
		name        string
		overlapSize int
	}{
		{"NoOverlap", 0},
		{"SmallOverlap", 100},
		{"LargeOverlap", 300},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategyClass,
					OverlapSize:     bm.overlapSize,
					EnableSplitting: true,
					MaxChunkSize:    1500,
				}

				_, err := chunker.ChunkByClass(ctx, allChunks, config)
				if err != nil {
					b.Fatalf("Nested class chunking failed: %v", err)
				}
			}
		})
	}
}

// Helper functions for creating test content

func createGoClassContent() string {
	return `type UserService struct {
    db         *sql.DB
    cache      *redis.Client
    logger     *log.Logger
    config     *Config
    validator  *validator.Validator
}

func NewUserService(db *sql.DB, cache *redis.Client, logger *log.Logger, config *Config) *UserService {
    return &UserService{
        db:        db,
        cache:     cache,
        logger:    logger,
        config:    config,
        validator: validator.New(),
    }
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
    if err := s.validator.Validate(user); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    // Implementation here
    return nil
}

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    // Implementation here
    return nil, nil
}`
}

func createPythonClassContent() string {
	return `class DataProcessor:
    """Advanced data processor with multiple processing strategies."""

    def __init__(self, config: Dict[str, Any]) -> None:
        self.config = config
        self._cache = {}
        self._metrics = defaultdict(int)

    def process_batch(self, data: List[Dict]) -> List[Dict]:
        """Process a batch of data items."""
        results = []
        for item in data:
            result = self._process_item(item)
            results.append(result)
        return results

    def _process_item(self, item: Dict) -> Dict:
        """Process a single data item."""
        # Apply transformations
        processed = self._transform(item)
        return processed

    def _transform(self, item: Dict) -> Dict:
        """Apply data transformations."""
        return item`
}

func createTypeScriptInterfaceContent() string {
	return `interface IHttpResponse {
    readonly status: number;
    readonly statusText: string;
    readonly headers: Record<string, string>;
    readonly body: ArrayBuffer;
    readonly url: string;
    readonly redirected: boolean;
    readonly type: ResponseType;
    readonly ok: boolean;

    clone(): IHttpResponse;
    blob(): Promise<Blob>;
   .formData(): Promise<FormData>;
    json(): Promise<any>;
    text(): Promise<string>;
}`
}

func createGoMethodChunks(qualifiedName string) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ChunkID:       "method-1",
			Name:          "CreateUser",
			Signature:     "func (s *UserService) CreateUser(ctx context.Context, user *User) error",
			Content:       "func (s *UserService) CreateUser(ctx context.Context, user *User) error { /* implementation */ }",
			Language:      mustCreateLanguage("go"),
			StartByte:     300,
			EndByte:       400,
			Type:          outbound.ConstructMethod,
			QualifiedName: qualifiedName + ".CreateUser",
		},
		{
			ChunkID:       "method-2",
			Name:          "GetUser",
			Signature:     "func (s *UserService) GetUser(ctx context.Context, id string) (*User, error)",
			Content:       "func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) { /* implementation */ }",
			Language:      mustCreateLanguage("go"),
			StartByte:     500,
			EndByte:       600,
			Type:          outbound.ConstructMethod,
			QualifiedName: qualifiedName + ".GetUser",
		},
	}
}

func createPythonMethodChunks(qualifiedName string) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ChunkID:       "method-1",
			Name:          "process_batch",
			Signature:     "def process_batch(self, data: List[Dict]) -> List[Dict]:",
			Content:       "def process_batch(self, data: List[Dict]) -> List[Dict]:\n        \"\"\"Process a batch of data items.\"\"\"\n        return []",
			Language:      mustCreateLanguage("python"),
			StartByte:     1500,
			EndByte:       1550,
			Type:          outbound.ConstructMethod,
			QualifiedName: qualifiedName + ".process_batch",
		},
		{
			ChunkID:       "method-2",
			Name:          "_process_item",
			Signature:     "def _process_item(self, item: Dict) -> Dict:",
			Content:       "def _process_item(self, item: Dict) -> Dict:\n        \"\"\"Process a single data item.\"\"\"\n        return item",
			Language:      mustCreateLanguage("python"),
			StartByte:     1600,
			EndByte:       1650,
			Type:          outbound.ConstructMethod,
			QualifiedName: qualifiedName + "._process_item",
		},
	}
}

func createTypeScriptMethodChunks(qualifiedName string) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ChunkID:       "method-1",
			Name:          "clone",
			Signature:     "clone(): IHttpResponse;",
			Content:       "clone(): IHttpResponse; // implementation",
			Language:      mustCreateLanguage("typescript"),
			StartByte:     1800,
			EndByte:       1850,
			Type:          outbound.ConstructMethod,
			QualifiedName: qualifiedName + ".clone",
		},
		{
			ChunkID:       "method-2",
			Name:          "json",
			Signature:     "json(): Promise<any>;",
			Content:       "json(): Promise<any>; // implementation",
			Language:      mustCreateLanguage("typescript"),
			StartByte:     1900,
			EndByte:       1950,
			Type:          outbound.ConstructMethod,
			QualifiedName: qualifiedName + ".json",
		},
	}
}

func createLargeClassContent(index int) string {
	template := `class Class%d:
    """Large class with multiple properties and methods."""

    def __init__(self):
        self.property_one = "value_one_%d"
        self.property_two = "value_two_%d"
        self.property_three = "value_three_%d"
        self.nested_dict = {
            "key1": "value1",
            "key2": "value2",
            "key3": "value3"
        }

    def method_one(self, param: str) -> str:
        """Method one with detailed implementation."""
        return f"{param}_processed_{%d}"

    def method_two(self, param: int) -> int:
        """Method two with different return type."""
        return param * %d

    def method_three(self, param: List[str]) -> Dict[str, str]:
        """Method three processing lists."""
        return {item: f"{item}_{%d}" for item in param}`

	return fmt.Sprintf(template, index, index, index, index, index, index, index)
}

func createMethodContent(classIndex, methodIndex int) string {
	template := `    def method%d(self, param):
        """Method %d from class %d."""
        return param * %d
`
	return fmt.Sprintf(template, methodIndex, methodIndex, classIndex, methodIndex*2)
}

func createBaseClassContent() string {
	return `from abc import ABC, abstractmethod
import logging

class BaseService(ABC):
    """Base service providing common functionality."""

    def __init__(self, logger: logging.Logger = None):
        self.logger = logger or logging.getLogger(__name__)
        self._initialized = False

    @abstractmethod
    def initialize(self) -> None:
        """Initialize the service."""
        pass

    def cleanup(self) -> None:
        """Cleanup resources."""
        self._initialized = False

    def is_initialized(self) -> bool:
        """Check if service is initialized."""
        return self._initialized`
}

func createDerivedClassContent(index int) string {
	template := `class Service%d(BaseService):
    """Service %d implementation."""

    def __init__(self, repository=None, logger=None):
        super().__init__(logger)
        self.repository = repository
        self.service_id = %d

    def initialize(self) -> None:
        """Initialize service %d."""
        self._initialized = True
        # Service %d initialized

    def process_data(self, data):
        """Process data specific to service %d."""
        if not self.is_initialized():
            raise RuntimeError("Service not initialized")
        return {"processed": True, "service_id": self.service_id, "data": data}`

	return fmt.Sprintf(template, index, index, index, index, index)
}

func createVeryLargeClassContent(index int) string {
	template := `class LargeClass%d:
    """Very large class designed to exceed max chunk size for testing splitting."""

    def __init__(self):
        self.property_001 = "value_001_%d"
        self.property_002 = "value_002_%d"
        self.property_003 = "value_003_%d"
        self.property_004 = "value_004_%d"
        self.property_005 = "value_005_%d"
        self.property_006 = "value_006_%d"
        self.property_007 = "value_007_%d"
        self.property_008 = "value_008_%d"
        self.property_009 = "value_009_%d"
        self.property_010 = "value_010_%d"

        # Large initialization
        self.large_data_structure = {k: f"value_{k}_{%d}" for k in range(1000)}

    def method_that_does_complex_processing_001(self, data):
        """Complex processing method 001."""
        result = []
        for item in data:
            processed = self._complex_transformation_001(item)
            result.append(processed)
        return result

    def method_that_does_complex_processing_002(self, data):
        """Complex processing method 002."""
        return [self._complex_transformation_002(x) for x in data]

    def _complex_transformation_001(self, item):
        """Internal transformation 001."""
        return {"original": item, "transformed": f"transformed_{item}_{%d}"}

    def _complex_transformation_002(self, item):
        """Internal transformation 002."""
        return f"{item.upper()}_PROCESSED_{%d}"`

	return fmt.Sprintf(
		template,
		index,
		index,
		index,
		index,
		index,
		index,
		index,
		index,
		index,
		index,
		index,
		index,
		index,
		index,
	)
}

func createComplexMethodContent(index int) string {
	template := `def process%d(self, data):
    """Complex method %d with multiple steps."""
    # Step 1: validate input
    if not data:
        raise ValueError("Data cannot be empty")

    # Step 2: transform data
    transformed = []
    for item in data:
        if isinstance(item, dict):
            transformed.append(self._process_dict_item(item, %d))
        elif isinstance(item, list):
            transformed.append(self._process_list_item(item, %d))
        else:
            transformed.append(str(item))

    # Step 3: filter results
    filtered = [x for x in transformed if x and len(x) > %d]

    # Step 4: finalize
    return {"count": len(filtered), "results": filtered}
`
	return fmt.Sprintf(template, index, index, index, index, index%3)
}

func createContainerClassContent() string {
	return `class Container:
    """Container class with nested classes and complex structure."""

    def __init__(self):
        self.main_config = {"debug": True, "version": "1.0"}
        self.connections = []

    class InnerClass1:
        def method1(self):
            return "inner_class_1_method_1"

    class InnerClass2:
        def method2(self):
            return "inner_class_2_method_2"

    class InnerClass3:
        def method3(self):
            return "inner_class_3_method_3"`
}

func createNestedClassContent(index int) string {
	template := `    class InnerClass%d:
        """Inner class %d with its own methods."""

        def __init__(self):
            self.value = %d

        def run(self):
            return f"Running InnerClass%d with value {self.value}`

	return fmt.Sprintf(template, index, index, index*10, index)
}

func createNestedMethodContent(index int) string {
	template := `        def nested_method%d(self, param):
            """Nested method %d."""
            return f"nested_result_%d_{param}"`

	return fmt.Sprintf(template, index, index, index)
}
