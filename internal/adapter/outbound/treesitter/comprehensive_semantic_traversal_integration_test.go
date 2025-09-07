package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEndSemanticExtraction(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	languages := []valueobject.Language{
		valueobject.NewLanguage(valueobject.LanguageGo),
		valueobject.NewLanguage(valueobject.LanguagePython),
		valueobject.NewLanguage(valueobject.LanguageJavaScript),
	}

	for _, lang := range languages {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		var code string
		switch lang.Name() {
		case valueobject.LanguageGo:
			code = `package services

import (
	"context"
	"database/sql"
	"errors"
)

// UserService handles user operations
type UserService struct {
	db *sql.DB
	logger Logger
}

// User represents a user entity
type User struct {
	ID    int64  ` + "`json:\"id\" db:\"id\"`" + `
	Name  string ` + "`json:\"name\" db:\"name\"`" + `
	Email string ` + "`json:\"email\" db:\"email\"`" + `
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, user *User) (*User, error) {
	// Implementation here
	return user, nil
}`
		case valueobject.LanguagePython:
			code = `from typing import Protocol, List, Optional
from abc import ABC, abstractmethod
import asyncio

class UserRepository(Protocol):
    def get_user(self, user_id: int) -> Optional['User']:
        ...

class User:
    def __init__(self, id: int, name: str, email: str):
        self.id = id
        self.name = name
        self.email = email

class UserService:
    def __init__(self, repository: UserRepository):
        self._repository = repository
    
    async def create_user(self, user_data: dict) -> User:
        """Create a new user asynchronously."""
        # Implementation here
        return User(**user_data)`
		case valueobject.LanguageJavaScript:
			code = `class UserService {
    constructor(repository) {
        this._repository = repository;
    }
    
    async createUser(userData) {
        // Implementation here
        return new User(userData);
    }
    
    static validateEmail(email) {
        return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
    }
}

const userService = new UserService(repository);
export { UserService, userService as default };`
		}

		tree, err := parser.Parse(ctx, []byte(code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		assert.Greater(t, len(chunks), 0)
		for _, chunk := range chunks {
			assert.NotEmpty(t, chunk.ID())
			assert.NotEmpty(t, chunk.Type())
			assert.NotEmpty(t, chunk.Content())
			assert.NotEmpty(t, chunk.Language())
			assert.NotNil(t, chunk.Metadata())
		}
	}
}

func TestMultiLanguageFileProcessing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	codeSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo): `package main
func main() {
	println("Hello, World!")
}`,
		valueobject.NewLanguage(valueobject.LanguagePython): `def main():
    print("Hello, World!")

if __name__ == "__main__":
    main()`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `function main() {
    console.log("Hello, World!");
}

main();`,
	}

	parserPool := make(map[valueobject.Language]outbound.CodeParser)
	for lang := range codeSamples {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)
		parserPool[lang] = parser
	}

	for lang, code := range codeSamples {
		parser := parserPool[lang]
		tree, err := parser.Parse(ctx, []byte(code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		assert.Greater(t, len(chunks), 0)
	}
}

func TestCrossLanguageConstructComparison(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `func processUser(user *User) error {
	return nil
}`

	pythonCode := `def process_user(user: User) -> Optional[Error]:
    return None`

	jsCode := `function processUser(user) {
    return null;
}`

	languages := []valueobject.Language{
		valueobject.NewLanguage(valueobject.LanguageGo),
		valueobject.NewLanguage(valueobject.LanguagePython),
		valueobject.NewLanguage(valueobject.LanguageJavaScript),
	}

	codeSamples := map[valueobject.Language]string{
		languages[0]: goCode,
		languages[1]: pythonCode,
		languages[2]: jsCode,
	}

	for _, lang := range languages {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(codeSamples[lang]))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		functionChunks := filterChunksByType(chunks, "function")
		assert.Greater(t, len(functionChunks), 0)

		for _, chunk := range functionChunks {
			assert.NotEmpty(t, chunk.Name())
			assert.NotNil(t, chunk.Parameters())
			assert.NotEmpty(t, chunk.ReturnType())
		}
	}
}

func TestClassStructExtractionAcrossLanguages(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `type User struct {
	ID int64
	Name string
}`

	pythonCode := `class User:
    def __init__(self, id: int, name: str):
        self.id = id
        self.name = name`

	jsCode := `class User {
    constructor(id, name) {
        this.id = id;
        this.name = name;
    }
}`

	languages := []valueobject.Language{
		valueobject.NewLanguage(valueobject.LanguageGo),
		valueobject.NewLanguage(valueobject.LanguagePython),
		valueobject.NewLanguage(valueobject.LanguageJavaScript),
	}

	codeSamples := map[valueobject.Language]string{
		languages[0]: goCode,
		languages[1]: pythonCode,
		languages[2]: jsCode,
	}

	for _, lang := range languages {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(codeSamples[lang]))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		classChunks := filterChunksByType(chunks, "class")
		structChunks := filterChunksByType(chunks, "struct")

		totalClassStructChunks := len(classChunks) + len(structChunks)
		assert.Greater(t, totalClassStructChunks, 0)

		for _, chunk := range classChunks {
			assert.NotEmpty(t, chunk.Name())
			assert.NotNil(t, chunk.Fields())
		}

		for _, chunk := range structChunks {
			assert.NotEmpty(t, chunk.Name())
			assert.NotNil(t, chunk.Fields())
		}
	}
}

func TestImportModuleHandlingDifferences(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `import (
	"context"
	"database/sql"
	"errors"
)`

	pythonCode := `from typing import Protocol, List, Optional
from abc import ABC, abstractmethod
import asyncio`

	jsCode := `import { UserRepository } from './repository';
import UserService from './UserService';`

	languages := []valueobject.Language{
		valueobject.NewLanguage(valueobject.LanguageGo),
		valueobject.NewLanguage(valueobject.LanguagePython),
		valueobject.NewLanguage(valueobject.LanguageJavaScript),
	}

	codeSamples := map[valueobject.Language]string{
		languages[0]: goCode,
		languages[1]: pythonCode,
		languages[2]: jsCode,
	}

	for _, lang := range languages {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(codeSamples[lang]))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		importChunks := filterChunksByType(chunks, "import")
		assert.Greater(t, len(importChunks), 0)

		for _, chunk := range importChunks {
			assert.NotEmpty(t, chunk.Name())
			assert.NotEmpty(t, chunk.Path())
		}
	}
}

func TestDocumentationExtractionConsistency(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `// UserService handles user operations
type UserService struct {
	db *sql.DB
}`

	pythonCode := `class UserService:
    """UserService handles user operations."""
    def __init__(self, db):
        self.db = db`

	jsCode := `/**
 * UserService handles user operations
 */
class UserService {
    constructor(db) {
        this.db = db;
    }
}`

	languages := []valueobject.Language{
		valueobject.NewLanguage(valueobject.LanguageGo),
		valueobject.NewLanguage(valueobject.LanguagePython),
		valueobject.NewLanguage(valueobject.LanguageJavaScript),
	}

	codeSamples := map[valueobject.Language]string{
		languages[0]: goCode,
		languages[1]: pythonCode,
		languages[2]: jsCode,
	}

	for _, lang := range languages {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(codeSamples[lang]))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		classChunks := filterChunksByType(chunks, "class")
		assert.Greater(t, len(classChunks), 0)

		for _, chunk := range classChunks {
			assert.NotEmpty(t, chunk.Documentation())
		}
	}
}

func TestParameterParsingStandardization(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `func processUser(ctx context.Context, user *User, opts ...Option) error {
	return nil
}`

	pythonCode := `def process_user(ctx: Context, user: User, *args, **kwargs) -> Error:
    pass`

	jsCode := `function processUser(ctx, user, ...opts) {
    return null;
}`

	languages := []valueobject.Language{
		valueobject.NewLanguage(valueobject.LanguageGo),
		valueobject.NewLanguage(valueobject.LanguagePython),
		valueobject.NewLanguage(valueobject.LanguageJavaScript),
	}

	codeSamples := map[valueobject.Language]string{
		languages[0]: goCode,
		languages[1]: pythonCode,
		languages[2]: jsCode,
	}

	for _, lang := range languages {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(codeSamples[lang]))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		functionChunks := filterChunksByType(chunks, "function")
		assert.Greater(t, len(functionChunks), 0)

		for _, chunk := range functionChunks {
			params := chunk.Parameters()
			assert.Greater(t, len(params), 0)

			for _, param := range params {
				assert.NotEmpty(t, param.Name())
				assert.NotEmpty(t, param.Type())
			}
		}
	}
}

func TestVisibilityDetectionAcrossLanguages(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `type User struct {
	PublicField  string
	privateField string
}

func PublicFunction() {}
func privateFunction() {}`

	pythonCode := `class User:
    def __init__(self):
        self.public_field = ""
        self._private_field = ""
    
    def public_method(self):
        pass
    
    def _private_method(self):
        pass`

	jsCode := `class User {
    constructor() {
        this.publicField = "";
        this._privateField = "";
    }
    
    publicMethod() {}
    
    _privateMethod() {}
}`

	languages := []valueobject.Language{
		valueobject.NewLanguage(valueobject.LanguageGo),
		valueobject.NewLanguage(valueobject.LanguagePython),
		valueobject.NewLanguage(valueobject.LanguageJavaScript),
	}

	codeSamples := map[valueobject.Language]string{
		languages[0]: goCode,
		languages[1]: pythonCode,
		languages[2]: jsCode,
	}

	for _, lang := range languages {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(codeSamples[lang]))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		for _, chunk := range chunks {
			assert.NotEmpty(t, chunk.Visibility())
		}
	}
}

func TestCompleteGoPackageProcessing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// UserService handles user operations
type UserService struct {
	db     *sql.DB
	logger Logger
}

// User represents a user entity
type User struct {
	ID    int64  ` + "`json:\"id\" db:\"id\"`" + `
	Name  string ` + "`json:\"name\" db:\"name\"`" + `
	Email string ` + "`json:\"email\" db:\"email\"`" + `
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, user *User) (*User, error) {
	if user.Email == "" {
		return nil, errors.New("email is required")
	}
	
	query := "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id"
	err := s.db.QueryRowContext(ctx, query, user.Name, user.Email).Scan(&user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id int64) (*User, error) {
	user := &User{}
	query := "SELECT id, name, email FROM users WHERE id = $1"
	err := s.db.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return user, nil
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(ctx context.Context, user *User) error {
	query := "UPDATE users SET name = $1, email = $2 WHERE id = $3"
	result, err := s.db.ExecContext(ctx, query, user.Name, user.Email, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return errors.New("user not found")
	}
	
	return nil
}

// DeleteUser removes a user by ID
func (s *UserService) DeleteUser(ctx context.Context, id int64) error {
	query := "DELETE FROM users WHERE id = $1"
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return errors.New("user not found")
	}
	
	return nil
}`

	lang := valueobject.NewLanguage(valueobject.LanguageGo)
	parser, err := factory.CreateParser(ctx, lang)
	require.NoError(t, err)

	tree, err := parser.Parse(ctx, []byte(goCode))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	assert.Greater(t, len(chunks), 4)

	structChunks := filterChunksByType(chunks, "struct")
	assert.Equal(t, 2, len(structChunks))

	methodChunks := filterChunksByType(chunks, "method")
	assert.Equal(t, 4, len(methodChunks))

	importChunks := filterChunksByType(chunks, "import")
	assert.Greater(t, len(importChunks), 0)
}

func TestPythonModuleWithClassHierarchies(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	pythonCode := `from abc import ABC, abstractmethod
from typing import List, Optional

class Entity(ABC):
    @abstractmethod
    def save(self):
        pass

class User(Entity):
    def __init__(self, id: int, name: str, email: str):
        self.id = id
        self.name = name
        self.email = email
    
    def save(self):
        # Save user to database
        pass
    
    def validate(self) -> bool:
        return bool(self.name and self.email)

class AdminUser(User):
    def __init__(self, id: int, name: str, email: str, permissions: List[str]):
        super().__init__(id, name, email)
        self.permissions = permissions
    
    def save(self):
        # Save admin user with permissions
        pass
    
    def has_permission(self, permission: str) -> bool:
        return permission in self.permissions`

	lang := valueobject.NewLanguage(valueobject.LanguagePython)
	parser, err := factory.CreateParser(ctx, lang)
	require.NoError(t, err)

	tree, err := parser.Parse(ctx, []byte(pythonCode))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	classChunks := filterChunksByType(chunks, "class")
	assert.Greater(t, len(classChunks), 2)

	for _, chunk := range classChunks {
		assert.NotEmpty(t, chunk.Name())
		assert.NotNil(t, chunk.Fields())
		assert.NotNil(t, chunk.Methods())
	}
}

func TestJavaScriptProjectWithES6Modules(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	jsCode := `import { Database } from './database.js';
import { Logger } from './logger.js';
import validator from './validator.js';

export class UserService {
    #db;
    #logger;
    
    constructor(database, logger) {
        this.#db = database;
        this.#logger = logger;
    }
    
    async createUser(userData) {
        if (!validator.validate(userData)) {
            throw new Error('Invalid user data');
        }
        
        try {
            const user = await this.#db.insert('users', userData);
            this.#logger.info(` + "`User created: ${user.id}`" + `);
            return user;
        } catch (error) {
            this.#logger.error(` + "`Failed to create user: ${error.message}`" + `);
            throw error;
        }
    }
    
    static get DEFAULT_CONFIG() {
        return {
            maxRetries: 3,
            timeout: 5000
        };
    }
}

export default UserService;`

	lang := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	parser, err := factory.CreateParser(ctx, lang)
	require.NoError(t, err)

	tree, err := parser.Parse(ctx, []byte(jsCode))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	classChunks := filterChunksByType(chunks, "class")
	assert.Greater(t, len(classChunks), 0)

	methodChunks := filterChunksByType(chunks, "method")
	assert.Greater(t, len(methodChunks), 0)

	importChunks := filterChunksByType(chunks, "import")
	assert.Greater(t, len(importChunks), 0)
}

func TestMixedCodebaseProcessing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	codeSamples := []struct {
		lang valueobject.Language
		code string
	}{
		{
			lang: valueobject.NewLanguage(valueobject.LanguageGo),
			code: `package main
func main() {
	println("Hello from Go")
}`,
		},
		{
			lang: valueobject.NewLanguage(valueobject.LanguagePython),
			code: `def main():
    print("Hello from Python")

if __name__ == "__main__":
    main()`,
		},
		{
			lang: valueobject.NewLanguage(valueobject.LanguageJavaScript),
			code: `function main() {
    console.log("Hello from JavaScript");
}

main();`,
		},
	}

	allChunks := []valueobject.SemanticCodeChunk{}
	for _, sample := range codeSamples {
		parser, err := factory.CreateParser(ctx, sample.lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(sample.code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		allChunks = append(allChunks, chunks...)
	}

	assert.Greater(t, len(allChunks), 0)

	// Verify we have chunks from all languages
	languageCount := make(map[string]int)
	for _, chunk := range allChunks {
		languageCount[chunk.Language()]++
	}

	assert.Greater(t, languageCount[valueobject.LanguageGo], 0)
	assert.Greater(t, languageCount[valueobject.LanguagePython], 0)
	assert.Greater(t, languageCount[valueobject.LanguageJavaScript], 0)
}

func TestLargeFileProcessing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// Generate a large file with 1000 functions
	goCode := "package main\n\n"
	for i := 0; i < 1000; i++ {
		goCode += fmt.Sprintf(`func function%d() {
	// This is a large function with many lines
	// Line 1
	// Line 2
	// Line 3
	// Line 4
	// Line 5
	// Line 6
	// Line 7
	// Line 8
	// Line 9
	// Line 10
}
`, i)
	}

	parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
	require.NoError(t, err)

	startTime := time.Now()
	tree, err := parser.Parse(ctx, []byte(goCode))
	require.NoError(t, err)
	parseTime := time.Since(startTime)

	converter := NewTreeSitterParseTreeConverter()
	startTime = time.Now()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)
	conversionTime := time.Since(startTime)

	extractor := NewSemanticCodeChunkExtractor()
	startTime = time.Now()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)
	extractionTime := time.Since(startTime)

	assert.Greater(t, len(chunks), 999)
	assert.Less(t, parseTime, 5*time.Second)
	assert.Less(t, conversionTime, 2*time.Second)
	assert.Less(t, extractionTime, 3*time.Second)
}

func TestDeeplyNestedCodeStructures(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	nestedGoCode := `package main

func deeplyNested() {
	if true {
		for i := 0; i < 10; i++ {
			switch i {
			case 1:
				if true {
					for j := 0; j < 5; j++ {
						switch j {
						case 1:
							if true {
								for k := 0; k < 3; k++ {
									switch k {
									case 1:
										println("deeply nested")
									}
								}
							}
						}
					}
				}
			}
		}
	}
}`

	parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
	require.NoError(t, err)

	tree, err := parser.Parse(ctx, []byte(nestedGoCode))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	assert.Greater(t, len(chunks), 0)
}

func TestSemanticChunkStructureConsistency(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	codeSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo):         `func testFunc() {}`,
		valueobject.NewLanguage(valueobject.LanguagePython):     `def test_func(): pass`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `function testFunc() {}`,
	}

	for lang, code := range codeSamples {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		for _, chunk := range chunks {
			assert.NotEmpty(t, chunk.ID())
			assert.NotEmpty(t, chunk.Type())
			assert.NotEmpty(t, chunk.Content())
			assert.NotEmpty(t, chunk.Language())
			assert.NotNil(t, chunk.Metadata())
			assert.NotEmpty(t, chunk.Visibility())
		}
	}
}

func TestStandardizedMetadataExtraction(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	codeSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo): `// Test function
func testFunc() {}`,
		valueobject.NewLanguage(valueobject.LanguagePython): `def test_func():
    """Test function"""
    pass`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `/**
 * Test function
 */
function testFunc() {}`,
	}

	for lang, code := range codeSamples {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		functionChunks := filterChunksByType(chunks, "function")
		assert.Greater(t, len(functionChunks), 0)

		for _, chunk := range functionChunks {
			metadata := chunk.Metadata()
			assert.NotNil(t, metadata)
			assert.NotEmpty(t, metadata.Name)
			assert.NotEmpty(t, metadata.Documentation)
			assert.NotEmpty(t, metadata.StartPosition)
			assert.NotEmpty(t, metadata.EndPosition)
		}
	}
}

func TestUniformErrorHandling(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	malformedCodeSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo):         `func testFunc( {}`,
		valueobject.NewLanguage(valueobject.LanguagePython):     `def test_func(:`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `function testFunc( {}`,
	}

	for lang, code := range malformedCodeSamples {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		_, err = parser.Parse(ctx, []byte(code))
		assert.Error(t, err)

		// Even with malformed code, the pipeline should handle errors gracefully
		// and not panic or corrupt state
	}
}

func TestIDGenerationAndHashingConsistency(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	code := `func testFunc() {}`

	parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
	require.NoError(t, err)

	tree, err := parser.Parse(ctx, []byte(code))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	functionChunks := filterChunksByType(chunks, "function")
	assert.Greater(t, len(functionChunks), 0)

	for _, chunk := range functionChunks {
		id := chunk.ID()
		assert.NotEmpty(t, id)
		assert.Len(t, id, 64) // SHA256 hash length
	}
}

func TestCrossLanguageDependencyTracking(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `import (
	"context"
	"database/sql"
)`

	pythonCode := `from typing import Protocol
import asyncio`

	jsCode := `import { UserRepository } from './repository';`

	codeSamples := []struct {
		lang valueobject.Language
		code string
	}{
		{valueobject.NewLanguage(valueobject.LanguageGo), goCode},
		{valueobject.NewLanguage(valueobject.LanguagePython), pythonCode},
		{valueobject.NewLanguage(valueobject.LanguageJavaScript), jsCode},
	}

	for _, sample := range codeSamples {
		parser, err := factory.CreateParser(ctx, sample.lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(sample.code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		importChunks := filterChunksByType(chunks, "import")
		assert.Greater(t, len(importChunks), 0)

		for _, chunk := range importChunks {
			deps := chunk.Dependencies()
			assert.NotNil(t, deps)
		}
	}
}

func TestMultiFileProjectParsing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	files := map[string]string{
		"main.go": `package main
import "./user"
func main() {
	u := user.NewUser()
	u.Print()
}`,
		"user/user.go": `package user
import "fmt"
type User struct {
	Name string
}
func NewUser() *User {
	return &User{Name: "test"}
}
func (u *User) Print() {
	fmt.Println(u.Name)
}`,
	}

	allChunks := []valueobject.SemanticCodeChunk{}
	for filePath, code := range files {
		lang := detectLanguage(filePath)
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		allChunks = append(allChunks, chunks...)
	}

	assert.Greater(t, len(allChunks), 0)
}

func TestPackageBoundaryDetection(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `package services
type UserService struct {}`

	pythonCode := `class UserService:
    pass`

	jsCode := `export class UserService {
    constructor() {}
}`

	codeSamples := []struct {
		lang valueobject.Language
		code string
	}{
		{valueobject.NewLanguage(valueobject.LanguageGo), goCode},
		{valueobject.NewLanguage(valueobject.LanguagePython), pythonCode},
		{valueobject.NewLanguage(valueobject.LanguageJavaScript), jsCode},
	}

	for _, sample := range codeSamples {
		parser, err := factory.CreateParser(ctx, sample.lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(sample.code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		classChunks := filterChunksByType(chunks, "class")
		assert.Greater(t, len(classChunks), 0)

		for _, chunk := range classChunks {
			assert.NotEmpty(t, chunk.Package())
		}
	}
}

func TestHierarchicalCodeOrganization(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `package main
import "fmt"

type OuterStruct struct {
	Inner InnerStruct
}

type InnerStruct struct {
	Value string
}

func (o *OuterStruct) Process() {
	o.Inner.ProcessInner()
}

func (i *InnerStruct) ProcessInner() {
	fmt.Println(i.Value)
}`

	parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
	require.NoError(t, err)

	tree, err := parser.Parse(ctx, []byte(goCode))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	assert.Greater(t, len(chunks), 0)

	// Verify hierarchical relationships are preserved
	structChunks := filterChunksByType(chunks, "struct")
	methodChunks := filterChunksByType(chunks, "method")

	assert.Equal(t, 2, len(structChunks))
	assert.Equal(t, 2, len(methodChunks))
}

func TestContextPreservationAcrossExtraction(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goCode := `package main
import "context"

func processData(ctx context.Context, data []byte) error {
	// Process data with context
	return nil
}`

	parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
	require.NoError(t, err)

	tree, err := parser.Parse(ctx, []byte(goCode))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	functionChunks := filterChunksByType(chunks, "function")
	assert.Greater(t, len(functionChunks), 0)

	for _, chunk := range functionChunks {
		assert.Equal(t, "processData", chunk.Name())
		params := chunk.Parameters()
		assert.Greater(t, len(params), 0)
		assert.Equal(t, "ctx", params[0].Name())
		assert.Equal(t, "context.Context", params[0].Type())
	}
}

func TestMetadataConsistencyValidation(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	codeSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo): `package main
// Test function
func testFunc() {}`,
		valueobject.NewLanguage(valueobject.LanguagePython): `# Test function
def test_func():
    pass`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `/**
 * Test function
 */
function testFunc() {}`,
	}

	for lang, code := range codeSamples {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, []byte(code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		functionChunks := filterChunksByType(chunks, "function")
		assert.Greater(t, len(functionChunks), 0)

		for _, chunk := range functionChunks {
			metadata := chunk.Metadata()
			assert.NotNil(t, metadata)
			assert.NotEmpty(t, metadata.Name)
			assert.NotEmpty(t, metadata.Documentation)
			assert.NotEmpty(t, metadata.StartPosition)
			assert.NotEmpty(t, metadata.EndPosition)
		}
	}
}

func TestConcurrentMultiLanguageParsing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	codeSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo):         `func test() {}`,
		valueobject.NewLanguage(valueobject.LanguagePython):     `def test(): pass`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `function test() {}`,
	}

	chunkChannels := make(map[valueobject.Language]<-chan []valueobject.SemanticCodeChunk)
	errorChannels := make(map[valueobject.Language]<-chan error)

	for lang, code := range codeSamples {
		chunkChan := make(chan []valueobject.SemanticCodeChunk, 1)
		errorChan := make(chan error, 1)
		chunkChannels[lang] = chunkChan
		errorChannels[lang] = errorChan

		go func(l valueobject.Language, c string) {
			parser, err := factory.CreateParser(ctx, l)
			if err != nil {
				errorChan <- err
				return
			}

			tree, err := parser.Parse(ctx, []byte(c))
			if err != nil {
				errorChan <- err
				return
			}

			converter := NewTreeSitterParseTreeConverter()
			domainTree, err := converter.ConvertToDomain(tree)
			if err != nil {
				errorChan <- err
				return
			}

			extractor := NewSemanticCodeChunkExtractor()
			chunks, err := extractor.Extract(ctx, domainTree)
			if err != nil {
				errorChan <- err
				return
			}

			chunkChan <- chunks
		}(lang, code)
	}

	for lang := range codeSamples {
		select {
		case chunks := <-chunkChannels[lang]:
			assert.Greater(t, len(chunks), 0)
		case err := <-errorChannels[lang]:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for parsing results")
		}
	}
}

func TestMemoryEfficiencyDuringLargeFileProcessing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// Create a large code sample
	largeCode := "package main\n\n"
	for i := 0; i < 5000; i++ {
		largeCode += fmt.Sprintf("func function%d() { return }\n", i)
	}

	parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
	require.NoError(t, err)

	// Measure memory before processing
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	tree, err := parser.Parse(ctx, []byte(largeCode))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	// Measure memory after processing
	runtime.GC()
	runtime.ReadMemStats(&m2)

	memoryUsed := m2.Alloc - m1.Alloc
	assert.Greater(t, len(chunks), 0)
	assert.Less(t, memoryUsed, uint64(50*1024*1024)) // Less than 50MB
}

func TestParseTimeComparisonAcrossLanguages(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	codeSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo): `func test() { 
			// Complex function
			for i := 0; i < 100; i++ {
				if i % 2 == 0 {
					println(i)
				}
			}
		}`,
		valueobject.NewLanguage(valueobject.LanguagePython): `def test():
			# Complex function
			for i in range(100):
				if i % 2 == 0:
					print(i)`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `function test() {
			// Complex function
			for (let i = 0; i < 100; i++) {
				if (i % 2 === 0) {
					console.log(i);
				}
			}
		}`,
	}

	parseTimes := make(map[valueobject.Language]time.Duration)

	for lang, code := range codeSamples {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		start := time.Now()
		_, err = parser.Parse(ctx, []byte(code))
		duration := time.Since(start)
		require.NoError(t, err)

		parseTimes[lang] = duration
	}

	// All parse times should be reasonable (less than 1 second)
	for _, duration := range parseTimes {
		assert.Less(t, duration, time.Second)
	}
}

func TestResourceCleanupValidation(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// Process multiple files to test resource cleanup
	for i := 0; i < 100; i++ {
		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)

		code := fmt.Sprintf("func test%d() {}", i)
		tree, err := parser.Parse(ctx, []byte(code))
		require.NoError(t, err)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)
		assert.Greater(t, len(chunks), 0)
	}

	// After processing, memory usage should not have exploded
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	assert.Less(t, m.Alloc, uint64(100*1024*1024)) // Less than 100MB
}

func TestParserPoolEfficiency(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// Repeatedly create parsers for the same language
	var parsers []outbound.CodeParser
	for i := 0; i < 50; i++ {
		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		parsers = append(parsers, parser)
	}

	// All parsers should be able to parse code successfully
	code := "func test() {}"
	for _, parser := range parsers {
		_, err := parser.Parse(ctx, []byte(code))
		assert.NoError(t, err)
	}
}

func TestFactoryOverheadMeasurement(t *testing.T) {
	ctx := context.Background()

	// Measure factory creation time
	start := time.Now()
	factory, err := NewTreeSitterParserFactory(ctx)
	duration := time.Since(start)
	require.NoError(t, err)
	assert.Less(t, duration, 5*time.Second)

	// Measure parser creation overhead
	lang := valueobject.NewLanguage(valueobject.LanguageGo)
	start = time.Now()
	parser, err := factory.CreateParser(ctx, lang)
	duration = time.Since(start)
	require.NoError(t, err)
	assert.Less(t, duration, time.Second)

	// Parser should work correctly after creation
	code := "func test() {}"
	_, err = parser.Parse(ctx, []byte(code))
	assert.NoError(t, err)
}

func TestGracefulHandlingOfMalformedCode(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	malformedSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo):         `func test( { }`,
		valueobject.NewLanguage(valueobject.LanguagePython):     `def test(: pass`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `function test( { }`,
	}

	for lang, code := range malformedSamples {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		// Should not panic, but may return error
		tree, err := parser.Parse(ctx, []byte(code))

		// Even if parsing fails, converter should handle gracefully
		converter := NewTreeSitterParseTreeConverter()
		domainTree, convertErr := converter.ConvertToDomain(tree)

		// Extractor should also handle gracefully
		extractor := NewSemanticCodeChunkExtractor()
		chunks, extractErr := extractor.Extract(ctx, domainTree)

		// At least one of these should have an error, but none should panic
		assert.True(t, err != nil || convertErr != nil || extractErr != nil)
		assert.NotNil(t, chunks) // Should return empty slice, not nil
	}
}

func TestLanguageDetectionFailureRecovery(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// Try to create parser for unsupported language
	unsupportedLang := valueobject.NewLanguage("unsupported")
	_, err = factory.CreateParser(ctx, unsupportedLang)
	assert.Error(t, err)

	// Factory should still work for supported languages after failure
	supportedLang := valueobject.NewLanguage(valueobject.LanguageGo)
	parser, err := factory.CreateParser(ctx, supportedLang)
	require.NoError(t, err)

	code := "func test() {}"
	_, err = parser.Parse(ctx, []byte(code))
	assert.NoError(t, err)
}

func TestParserCreationFailureScenarios(t *testing.T) {
	ctx := context.Background()

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	factory, err := NewTreeSitterParserFactory(cancelledCtx)
	if err != nil {
		// If factory creation fails with cancelled context, that's acceptable
		return
	}

	lang := valueobject.NewLanguage(valueobject.LanguageGo)
	_, err = factory.CreateParser(cancelledCtx, lang)
	assert.Error(t, err)

	// Factory should still work with valid context
	factory, err = NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	parser, err := factory.CreateParser(ctx, lang)
	require.NoError(t, err)

	code := "func test() {}"
	_, err = parser.Parse(ctx, []byte(code))
	assert.NoError(t, err)
}

func TestParseTreeConversionErrorHandling(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
	require.NoError(t, err)

	// Create a tree with malformed code
	malformedCode := "func test( { }"
	tree, err := parser.Parse(ctx, []byte(malformedCode))

	// Conversion should handle errors gracefully
	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)

	// Even with conversion errors, should not panic
	assert.Error(t, err)
	assert.NotNil(t, domainTree)
}

func TestExtractionFailureIsolation(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
	require.NoError(t, err)

	// Parse valid code
	validCode := "func test() {}"
	tree, err := parser.Parse(ctx, []byte(validCode))
	require.NoError(t, err)

	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(tree)
	require.NoError(t, err)

	// Simulate a corrupted domain tree
	extractor := NewSemanticCodeChunkExtractor()
	chunks, err := extractor.Extract(ctx, nil) // Pass nil to simulate corruption

	// Should handle gracefully without panic
	assert.Error(t, err)
	assert.NotNil(t, chunks) // Should return empty slice, not nil
}

func TestCrossLanguageErrorConsistency(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	malformedSamples := map[valueobject.Language]string{
		valueobject.NewLanguage(valueobject.LanguageGo):         `func test( { }`,
		valueobject.NewLanguage(valueobject.LanguagePython):     `def test(: pass`,
		valueobject.NewLanguage(valueobject.LanguageJavaScript): `function test( { }`,
	}

	errorTypes := []string{}
	for lang, code := range malformedSamples {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, _ := parser.Parse(ctx, []byte(code))
		converter := NewTreeSitterParseTreeConverter()
		_, convertErr := converter.ConvertToDomain(tree)

		extractor := NewSemanticCodeChunkExtractor()
		_, extractErr := extractor.Extract(ctx, nil)

		// Collect error types for consistency checking
		if convertErr != nil {
			errorTypes = append(errorTypes, fmt.Sprintf("%T", convertErr))
		}
		if extractErr != nil {
			errorTypes = append(errorTypes, fmt.Sprintf("%T", extractErr))
		}
	}

	// All languages should produce consistent error handling patterns
	assert.Greater(t, len(errorTypes), 0)
}

func filterChunksByType(chunks []valueobject.SemanticCodeChunk, chunkType string) []valueobject.SemanticCodeChunk {
	var filtered []valueobject.SemanticCodeChunk
	for _, chunk := range chunks {
		if chunk.Type() == chunkType {
			filtered = append(filtered, chunk)
		}
	}
	return filtered
}

func detectLanguage(filePath string) valueobject.Language {
	// Simplified language detection based on file extension
	switch {
	case strings.HasSuffix(filePath, ".go"):
		return valueobject.NewLanguage(valueobject.LanguageGo)
	case strings.HasSuffix(filePath, ".py"):
		return valueobject.NewLanguage(valueobject.LanguagePython)
	case strings.HasSuffix(filePath, ".js"):
		return valueobject.NewLanguage(valueobject.LanguageJavaScript)
	default:
		return valueobject.NewLanguage(valueobject.LanguageGo) // default
	}
}
