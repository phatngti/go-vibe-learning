# Swag Comment Generator 🔧

Auto-generate [Swaggo](https://github.com/swaggo/swag) annotation comments for Go HTTP handlers directly from your editor.

## Features

- **`/genswagger` slash command** — Type `/genswagger` above a handler function and select from the dropdown to insert a fully populated Swaggo comment block
- **Context-aware parsing** — Automatically detects function name, HTTP method, route path, request/response DTOs, path/query parameters, and auth middleware
- **Multiple variants** — `/genswagger:get`, `/genswagger:post`, `/genswagger:put`, `/genswagger:delete`, `/genswagger:empty`
- **Lightbulb quick-fix** — Shows a 💡 when cursor is above an undocumented handler
- **Command palette** — `Cmd+Shift+Alt+S` to generate from keyboard
- **Tab-stop navigation** — Navigate through placeholder values with Tab
- **Configurable** — Default tags, error types, security schemes, and more

## Supported Framework

Currently supports the **Echo** framework (`labstack/echo/v4`).

See [Framework Guide](docs/FRAMEWORK_GUIDE.md) for how to add Gin, Chi, or net/http support.

## Installation

### From Source (Development)

```bash
cd tools/swag-comment-gen
npm install
npm run compile
```

Then press **F5** in VS Code to launch the Extension Development Host.

### From VSIX

```bash
npm run package
# Install the generated .vsix file:
code --install-extension swag-comment-gen-1.0.0.vsix
```

## Usage

### Slash Command (recommended)

1. Place your cursor on the blank line above a handler function
2. Type `/genswagger`
3. Select from the IntelliSense dropdown:

| Command | Description |
|---|---|
| `/genswagger` | Auto-detect handler and generate full annotation |
| `/genswagger:get` | Force GET handler template |
| `/genswagger:post` | Force POST handler template with body `@Param` |
| `/genswagger:put` | Force PUT handler template with body `@Param` |
| `/genswagger:delete` | Force DELETE handler template |
| `/genswagger:empty` | Bare-bones template with all placeholders |

### Keyboard Shortcut

- **Mac:** `Cmd+Shift+Alt+S`
- **Windows/Linux:** `Ctrl+Shift+Alt+S`

### Lightbulb

Place cursor on the line above an undocumented `func` → click the 💡 or press `Cmd+.`

## Configuration

| Setting | Default | Description |
|---|---|---|
| `swagCommentGen.triggerPrefix` | `/genswagger` | Slash command prefix |
| `swagCommentGen.defaultTag` | `""` | Default `@Tags` (empty = auto-detect) |
| `swagCommentGen.defaultErrorType` | `response.ErrorResponse` | Type for `@Failure` |
| `swagCommentGen.securityScheme` | `BearerAuth` | Name for `@Security` |
| `swagCommentGen.includeSecurity` | `true` | Auto-add `@Security` when auth detected |
| `swagCommentGen.useTabAlignment` | `true` | Align columns with tabs |
| `swagCommentGen.postSuccessCode` | `201` | Success code for POST handlers |

## Cross-IDE Support

### GoLand / JetBrains

Import `exports/goland/SwagCommentTemplates.xml`:
1. Settings → Editor → Live Templates → Import
2. Type `sget`, `spost`, `sgetid`, or `swag` and press Tab

### Neovim

Copy `exports/neovim/swaggo.lua` to your LuaSnip snippets directory:
```lua
-- In your LuaSnip config:
require("luasnip.loaders.from_lua").load({ paths = "path/to/exports/neovim/" })
```

Triggers: `sget`, `spost`, `sgetid`, `swag`, `sdel`

## How It Works

```
User types /genswagger
        │
        ▼
┌──────────────────────┐
│ CompletionProvider    │  Detects /genswagger text, shows dropdown
│ (trigger: '/')       │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ GoFunctionParser      │  Scans below cursor for func declaration
│ + RouteScanner        │  Finds route, method, DTOs, params, auth
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ TemplateEngine        │  Builds the Swaggo comment block with
│                       │  tab stops for unresolved fields
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ SnippetString insert  │  Replaces /genswagger with the comment
│ at cursor position    │  User navigates with Tab
└──────────────────────┘
```

## License

MIT
