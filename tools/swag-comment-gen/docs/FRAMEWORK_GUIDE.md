# Framework Support Guide

This extension currently supports **Echo** (`labstack/echo`) handler patterns. This guide explains how to add support for other Go web frameworks.

## Current Architecture

The parser pipeline has three stages:

```
GoFunctionParser → RouteScanner → TemplateEngine
```

Each framework needs its own implementations of the first two stages. The `TemplateEngine` is framework-agnostic (Swaggo annotation format is the same regardless of router).

---

## Adding Gin Support

### 1. Function Signature Pattern

**Echo:**
```go
func (h *Handler) listUsers(c echo.Context) error {
```

**Gin:**
```go
func (h *Handler) listUsers(c *gin.Context) {
```

**Parser change:** Add a regex for Gin's pattern:
```typescript
// In goFunctionParser.ts
const GIN_FUNC = /^func\s+\((\w+)\s+\*?(\w+)\)\s+(\w+)\s*\((\w+)\s+\*gin\.Context\)/;
```

### 2. Route Registration Pattern

**Echo:**
```go
g.GET("/users", h.listUsers)
```

**Gin:**
```go
r.GET("/users", h.listUsers)
// or with RouterGroup:
api := r.Group("/api")
api.GET("/users", h.listUsers)
```

**RouteScanner change:** Gin uses the same `g.METHOD("/path", handler)` format, so the existing regex works. The only difference is that Gin doesn't have a `Pattern()` method — the prefix comes from `Group()`.

### 3. Body DTO Detection

**Echo:**
```go
c.Get("dto").(*dto.LoginRequestDTO)   // Via middleware
c.Bind(&req)                           // Direct binding
```

**Gin:**
```go
c.ShouldBindJSON(&req)
c.BindJSON(&req)
c.ShouldBind(&req)
```

**Parser change:**
```typescript
const GIN_BODY_DTO = /c\.(?:ShouldBind|Bind|ShouldBindJSON|BindJSON)\s*\(\s*&(\w+)\s*\)/;
```

### 4. Query/Path Param Detection

**Echo:**
```go
c.QueryParam("limit")
c.Param("id")
```

**Gin:**
```go
c.Query("limit")
c.DefaultQuery("limit", "50")
c.Param("id")
```

**Parser change:**
```typescript
const GIN_QUERY = /c\.(?:Query|DefaultQuery)\s*\(\s*"(\w+)"/g;
const GIN_PARAM = /c\.Param\s*\(\s*"(\w+)"\s*\)/g;  // Same as Echo
```

### 5. Response DTO Detection

**Echo:**
```go
c.JSON(http.StatusOK, dto.Response{})
```

**Gin:**
```go
c.JSON(http.StatusOK, dto.Response{})  // Same!
```

No change needed — both use `c.JSON(...)`.

---

## Adding Chi Support

### 1. Function Signature Pattern

**Chi uses `net/http` signature:**
```go
func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
```

**Parser change:**
```typescript
const CHI_FUNC = /^func\s+\((\w+)\s+\*?(\w+)\)\s+(\w+)\s*\((\w+)\s+http\.ResponseWriter\s*,\s*(\w+)\s+\*http\.Request\)/;
```

### 2. Route Registration

**Chi:**
```go
r.Get("/users", h.listUsers)
r.Post("/users", h.createUser)
// or with route group:
r.Route("/api", func(r chi.Router) {
    r.Get("/users", h.listUsers)
})
```

**RouteScanner change:** Chi uses lowercase methods (`r.Get`, `r.Post`) and nested `Route()` closures for grouping — significantly different from Echo.

### 3. Body DTO Detection

**Chi/net-http:**
```go
json.NewDecoder(r.Body).Decode(&req)
```

**Parser change:**
```typescript
const CHI_BODY = /json\.NewDecoder\s*\(\s*\w+\.Body\s*\)\s*\.Decode\s*\(\s*&(\w+)\s*\)/;
```

### 4. Query/Path Param Detection

**Chi:**
```go
r.URL.Query().Get("limit")
chi.URLParam(r, "id")
```

**Parser change:**
```typescript
const CHI_QUERY = /\.URL\.Query\(\)\.Get\s*\(\s*"(\w+)"\s*\)/g;
const CHI_PARAM = /chi\.URLParam\s*\(\s*\w+\s*,\s*"(\w+)"\s*\)/g;
```

---

## Implementation Steps for New Frameworks

1. **Create a new parser file:** `src/parser/ginParser.ts` or `src/parser/chiParser.ts`
2. **Implement the `FrameworkParser` interface:**
   ```typescript
   interface FrameworkParser {
     detectFramework(documentText: string): boolean;
     parseFunctionSignature(line: string): FuncInfo | null;
     extractBodyDTO(funcBody: string): string | null;
     extractQueryParams(funcBody: string): ParamInfo[];
     extractPathParams(funcBody: string): ParamInfo[];
     extractResponseDTO(funcBody: string): string | null;
   }
   ```
3. **Add framework detection** in `goFunctionParser.ts`:
   ```typescript
   function detectFramework(documentText: string): 'echo' | 'gin' | 'chi' | 'unknown' {
     if (documentText.includes('"github.com/labstack/echo')) return 'echo';
     if (documentText.includes('"github.com/gin-gonic/gin"')) return 'gin';
     if (documentText.includes('"github.com/go-chi/chi"')) return 'chi';
     return 'unknown';
   }
   ```
4. **Route the parser** based on detected framework
5. **Add tests** with sample handler files for each framework

---

## Contributing

If you want to contribute framework support:

1. Fork the repo
2. Create `src/parser/<framework>Parser.ts`
3. Add test fixtures in `src/test/fixtures/<framework>_handler.go`
4. Submit a PR with tests passing

The `TemplateEngine` and `providers/` modules should not need any changes — they work with the framework-agnostic `HandlerMetadata` interface.
