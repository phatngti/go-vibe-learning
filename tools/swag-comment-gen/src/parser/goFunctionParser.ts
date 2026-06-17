/**
 * Go Function Parser — Extracts handler metadata from Go source code.
 *
 * Parses the Go file to extract function name, receiver, parameters,
 * DTO types, query/path/header/formData params, file uploads, response headers,
 * deprecation markers, and route information.
 *
 * Supports multiple frameworks:
 *   - Echo (echo.Context)
 *   - Gin (*gin.Context)
 *   - Standard library (http.ResponseWriter, *http.Request)
 *   - Chi (chi router with stdlib signatures)
 *
 * Covers ALL swaggo/swag Declarative Comments:
 * https://github.com/swaggo/swag#api-operation
 */

import {
  HandlerMetadata,
  HttpMethod,
  ParamInfo,
  SlashVariant,
  ResponseHeaderInfo,
  FailureInfo,
} from './types';
import { findRouteForHandler } from './routeScanner';

/** Regex patterns for Go function parsing */
const PATTERNS = {
  // ── Function signature patterns ──────────────────────────────────────

  /** Match method with receiver: func (h *Handler) name(c echo.Context) error */
  funcWithReceiver:
    /^func\s+\((\w+)\s+\*?(\w+)\)\s+(\w+)\s*\(([^)]*)\)\s*([^\s{]*)\s*\{/,

  /** Match standalone function: func name(params...) returnType { */
  funcStandalone: /^func\s+(\w+)\s*\(([^)]*)\)\s*([^\s{]*)\s*\{/,

  // ── Framework detection ──────────────────────────────────────────────

  /** Match echo.Context in parameter types */
  echoContext: /echo\.Context/,

  /** Match gin.Context in parameter types */
  ginContext: /\*?gin\.Context/,

  /** Match stdlib handler: (w http.ResponseWriter, r *http.Request) */
  stdlibHandler: /http\.ResponseWriter.*\*?http\.Request/,

  // ── Body DTO extraction ──────────────────────────────────────────────

  /** Match body DTO extraction: c.Get("dto").(*dto.LoginRequestDTO) — Echo custom */
  bodyDTOGet: /c\.Get\s*\(\s*"dto"\s*\)\s*\.\s*\(\s*\*?([\w.]+)\s*\)/,

  /** Match Echo Bind: c.Bind(&req) or if err := c.Bind(&req) */
  echoBind: /c\.Bind\s*\(\s*&(\w+)\s*\)/,

  /** Match Gin ShouldBindJSON/BindJSON: c.ShouldBindJSON(&req) */
  ginBind: /c\.(?:ShouldBind|Bind)(?:JSON|XML|YAML|Uri|Query|Header)?\s*\(\s*&(\w+)\s*\)/,

  /** Match Gin ShouldBind: c.ShouldBind(&req) */
  ginShouldBind: /c\.ShouldBind\s*\(\s*&(\w+)\s*\)/,

  /** Match stdlib JSON decode: json.NewDecoder(r.Body).Decode(&req) */
  stdlibJsonDecode: /json\.NewDecoder\s*\(\s*\w+\.Body\s*\)\.Decode\s*\(\s*&(\w+)\s*\)/,

  /** Match variable type declaration for bind target: var req dto.LoginRequest */
  varTypeDecl: /var\s+(\w+)\s+([\w.*]+)/g,

  /** Match short variable decl with composite literal: req := dto.LoginRequest{} */
  shortVarDecl: /(\w+)\s*:=\s*([\w.]+)\s*\{/g,

  // ── Query param extraction ───────────────────────────────────────────

  /** Match query param extraction: c.QueryParam("limit") — Echo */
  queryParam: /c\.QueryParam\s*\(\s*"(\w+)"\s*\)/g,

  /** Match gin query: c.Query("limit") or c.DefaultQuery("limit", "10") */
  ginQueryParam: /c\.(?:Query|DefaultQuery)\s*\(\s*"(\w+)"/g,

  /** Match stdlib query: r.URL.Query().Get("limit") or q.Get("limit") */
  stdlibQueryParam: /\.(?:URL\.)?Query\(\)\.Get\s*\(\s*"(\w+)"\s*\)/g,

  /** Match r.FormValue("name") — stdlib */
  stdlibFormValue: /r\.FormValue\s*\(\s*"(\w+)"\s*\)/g,

  // ── Path param extraction ────────────────────────────────────────────

  /** Match path param extraction: c.Param("id") — Echo/Gin */
  pathParam: /c\.Param\s*\(\s*"(\w+)"\s*\)/g,

  /** Match chi path param: chi.URLParam(r, "id") */
  chiPathParam: /chi\.URLParam\s*\(\s*\w+\s*,\s*"(\w+)"\s*\)/g,

  /** Match mux path param: mux.Vars(r)["id"] */
  muxPathParam: /mux\.Vars\s*\(\s*\w+\s*\)\s*\[\s*"(\w+)"\s*\]/g,

  // ── Header param extraction ──────────────────────────────────────────

  /** Match header extraction: c.Request().Header.Get("X-Custom") — Echo */
  echoHeaderGet: /c\.Request\(\)\.Header\.Get\s*\(\s*"([^"]+)"\s*\)/g,

  /** Match header extraction: c.GetHeader("X-Custom") — Gin */
  ginHeaderGet: /c\.GetHeader\s*\(\s*"([^"]+)"\s*\)/g,

  /** Match header extraction: r.Header.Get("X-Custom") — stdlib */
  stdlibHeaderGet: /r\.Header\.Get\s*\(\s*"([^"]+)"\s*\)/g,

  // ── FormData param extraction ────────────────────────────────────────

  /** Match form value: c.FormValue("name") — Echo */
  echoFormValue: /c\.FormValue\s*\(\s*"(\w+)"\s*\)/g,

  /** Match form value: c.PostForm("name") — Gin */
  ginFormValue: /c\.PostForm\s*\(\s*"(\w+)"\s*\)/g,

  /** Match file upload: c.FormFile("file") — Echo */
  echoFormFile: /c\.FormFile\s*\(\s*"(\w+)"\s*\)/g,

  /** Match file upload: c.FormFile("file") — Gin (same API) */
  ginFormFile: /c\.FormFile\s*\(\s*"(\w+)"\s*\)/g,

  /** Match stdlib multipart: r.FormFile("file") */
  stdlibFormFile: /r\.FormFile\s*\(\s*"(\w+)"\s*\)/g,

  /** Match stdlib multipart form: r.MultipartForm */
  stdlibMultipartForm: /r\.(?:ParseMultipartForm|MultipartForm)/,

  // ── Type inference (strconv wrapping) ────────────────────────────────

  /** Match strconv.Atoi wrapping (implies int type): strconv.Atoi(c.QueryParam("limit")) */
  intQueryParam: /strconv\.Atoi\s*\(\s*c\.QueryParam\s*\(\s*"(\w+)"\s*\)\s*\)/g,

  /** Match strconv.Atoi for path param */
  intPathParam: /strconv\.Atoi\s*\(\s*c\.Param\s*\(\s*"(\w+)"\s*\)\s*\)/g,

  /** Match strconv.ParseBool for query param */
  boolQueryParam: /strconv\.ParseBool\s*\(\s*c\.QueryParam\s*\(\s*"(\w+)"\s*\)\s*\)/g,

  /** Match strconv.ParseFloat for query param */
  floatQueryParam: /strconv\.ParseFloat\s*\(\s*c\.QueryParam\s*\(\s*"(\w+)"\s*\)\s*\)/g,

  // ── Response detection ───────────────────────────────────────────────

  /** Match response DTO in c.JSON calls: dto.SomeResponse{...} or &dto.SomeResponse{...} */
  responseDTOInJSON: /c\.JSON\s*\([^,]+,\s*(?:response\.Wrap[^,]*,\s*[^,]*,\s*)?&?([\w]+\.[\w]+)\s*\{/,

  /** Match response DTO in response.Wrap: response.Wrap(..., ..., dto.ListUsersResponse{...}) */
  responseDTOInWrap:
    /response\.Wrap\s*\([^,]+,\s*[^,]+,\s*\n?\s*(?:&)?([\w]+\.[\w]+)\s*\{/,

  /** Match gin JSON response: c.JSON(200, gin.H{...}) or c.JSON(200, resp) */
  ginResponseDTO: /c\.JSON\s*\(\s*[^,]+,\s*(?:&)?([\w]+\.[\w]+)\s*[{)]/,

  /** Match XML response: c.XML(...) — Echo/Gin */
  xmlResponse: /c\.XML\s*\(/,

  /** Match plain text response: c.String(...) — Echo */
  stringResponse: /c\.String\s*\(/,

  /** Match HTML response: c.HTML(...) — Echo/Gin */
  htmlResponse: /c\.HTML\s*\(/,

  /** Match No Content response: c.NoContent(204) or http.StatusNoContent */
  noContentResponse: /c\.NoContent\s*\(\s*(?:http\.StatusNoContent|204)\s*\)|http\.StatusNoContent/,

  /** Match array response: []dto.SomeType or []*dto.SomeType in return */
  arrayResponseType: /\[\]\*?([\w.]+)\s*\{/,

  /** Match make([]...) for array responses */
  makeSliceResponse: /make\s*\(\s*\[\]\*?([\w.]+)/,

  // ── Response headers ─────────────────────────────────────────────────

  /** Match response header set: c.Response().Header().Set("X-Request-Id", ...) — Echo */
  echoResponseHeader: /c\.Response\(\)\.Header\(\)\.Set\s*\(\s*"([^"]+)"\s*,/g,

  /** Match response header set: c.Header("X-Total-Count", ...) — Gin */
  ginResponseHeader: /c\.Header\s*\(\s*"([^"]+)"\s*,/g,

  /** Match response header set: w.Header().Set("X-Request-Id", ...) — stdlib */
  stdlibResponseHeader: /w\.Header\(\)\.Set\s*\(\s*"([^"]+)"\s*,/g,

  // ── Error/status code detection ──────────────────────────────────────

  /** Match HTTP status codes in function body */
  httpStatusNotFound: /http\.StatusNotFound|404/,
  httpStatusForbidden: /http\.StatusForbidden|403/,
  httpStatusConflict: /http\.StatusConflict|409/,
  httpStatusUnprocessable: /http\.StatusUnprocessableEntity|422/,
  httpStatusBadRequest: /http\.StatusBadRequest|400/,
  httpStatusUnauthorized: /http\.StatusUnauthorized|401/,

  // ── Auth & security ──────────────────────────────────────────────────

  /** Match auth middleware in function body (as a fallback) */
  authInBody: /authenticate\.|[Aa]uthorize\(\)/,

  // ── Route path params ────────────────────────────────────────────────

  /** Route path params pattern: /users/:id → extracts "id" */
  routePathParam: /:([\w]+)/g,

  // ── Tag inference ────────────────────────────────────────────────────

  /** Existing @Tags annotation in the same file */
  existingTag: /\/\/\s*@Tags\s+(\S+)/,

  // ── Deprecated detection ─────────────────────────────────────────────

  /** Match // Deprecated: comment above function */
  deprecatedComment: /\/\/\s*Deprecated:/i,

  /** Match //go:deprecated directive */
  deprecatedDirective: /\/\/go:deprecated/i,
};

/**
 * Known header names that should NOT become @Param header annotations.
 * These are standard headers managed by the framework, not API params.
 */
const IGNORED_HEADERS = new Set([
  'Content-Type',
  'Content-Length',
  'Accept',
  'Accept-Encoding',
  'Accept-Language',
  'Connection',
  'Host',
  'User-Agent',
  'Cache-Control',
  'Cookie',
  'Set-Cookie',
]);

/**
 * Known response headers that should NOT become @Header annotations.
 */
const IGNORED_RESPONSE_HEADERS = new Set([
  'Content-Type',
  'Content-Length',
  'Content-Encoding',
  'Connection',
  'Set-Cookie',
]);

/**
 * HTTP status code to description mapping.
 */
const STATUS_DESCRIPTIONS: Record<number, string> = {
  400: 'Bad request',
  401: 'Unauthorized',
  403: 'Forbidden',
  404: 'Not found',
  409: 'Conflict',
  422: 'Unprocessable entity',
  500: 'Internal error',
};

/**
 * Parse a Go handler function and extract all metadata needed for Swagger annotation.
 *
 * @param documentText - Full text of the Go source file.
 * @param cursorLine - 0-indexed line number where the user invoked /genswagger.
 * @param variant - Which /genswagger variant was used.
 * @returns HandlerMetadata or null if no function found.
 */
export function parseGoFunction(
  documentText: string,
  cursorLine: number,
  variant: SlashVariant
): HandlerMetadata | null {
  const lines = documentText.split('\n');

  // 1. Find the function declaration below the cursor
  const funcInfo = findFunctionBelow(lines, cursorLine);
  if (!funcInfo) {
    return null;
  }

  // 2. Extract function body
  const funcBody = extractFunctionBody(lines, funcInfo.line);

  // 3. Find route mapping from RegisterRoutes()
  const routeMapping = findRouteForHandler(documentText, funcInfo.name);

  // 4. Determine HTTP method (from route or variant override)
  let httpMethod: HttpMethod | null = null;
  if (variant !== 'auto' && variant !== 'empty') {
    httpMethod = variant.toUpperCase() as HttpMethod;
  } else if (routeMapping) {
    httpMethod = routeMapping.method;
  }

  // 5. Extract body DTO
  const bodyDTOType = extractBodyDTO(funcBody, documentText);

  // 6. Extract response DTO
  const responseDTOType = extractResponseDTO(funcBody);

  // 7. Extract query params
  const queryParams = extractQueryParams(funcBody);

  // 8. Extract path params from function body AND route pattern
  const pathParams = extractPathParams(funcBody, routeMapping?.fullPath ?? null);

  // 9. Extract header params
  const headerParams = extractHeaderParams(funcBody);

  // 10. Extract formData params and file uploads
  const { formDataParams, fileParams, hasFileUpload } = extractFormDataParams(funcBody);

  // 11. Extract response headers
  const responseHeaders = extractResponseHeaders(funcBody);

  // 12. Detect deprecation
  const isDeprecated = detectDeprecation(lines, funcInfo.line);

  // 13. Detect Accept/Produce MIME types
  const { acceptMimeTypes, produceMimeTypes } = detectMimeTypes(funcBody, hasFileUpload, bodyDTOType);

  // 14. Detect additional failure codes
  const additionalFailures = detectFailureCodes(funcBody);

  // 15. Detect array response
  const isArrayResponse = detectArrayResponse(funcBody);

  // 16. Detect no-body response (204)
  const noBodyResponse = PATTERNS.noContentResponse.test(funcBody);

  // 17. Determine auth
  const hasAuth = routeMapping?.hasAuth ?? PATTERNS.authInBody.test(funcBody);

  // 18. Infer tag
  const inferredTag = inferTag(documentText, lines);

  // 19. Generate operation ID
  const operationId = generateOperationId(funcInfo.name, funcInfo.receiver);

  return {
    functionName: funcInfo.name,
    receiverType: funcInfo.receiver,
    isEchoHandler: funcInfo.isEcho,
    isGinHandler: funcInfo.isGin,
    isStdlibHandler: funcInfo.isStdlib,
    isHttpHandler: funcInfo.isEcho || funcInfo.isGin || funcInfo.isStdlib,
    httpMethod,
    routePath: routeMapping?.fullPath ?? null,
    hasAuth,
    bodyDTOType,
    responseDTOType,
    pathParams,
    queryParams,
    headerParams,
    formDataParams,
    hasFileUpload,
    fileParams,
    responseHeaders,
    isDeprecated,
    acceptMimeTypes,
    produceMimeTypes,
    additionalFailures,
    isArrayResponse,
    noBodyResponse,
    additionalRouters: [],
    securityScopes: [],
    functionLine: funcInfo.line,
    validationDTOType: routeMapping?.validationDTO ?? null,
    inferredTag,
    operationId,
  };
}

// ═══════════════════════════════════════════════════════════════════════════
// Function Discovery
// ═══════════════════════════════════════════════════════════════════════════

/** Info about a found function declaration */
interface FuncInfo {
  name: string;
  receiver: string | null;
  isEcho: boolean;
  isGin: boolean;
  isStdlib: boolean;
  line: number;
}

/**
 * Scan downward from the cursor to find the first function declaration.
 * Skips blank lines and comments. Searches up to 10 lines down.
 */
function findFunctionBelow(lines: string[], fromLine: number): FuncInfo | null {
  const maxSearch = Math.min(fromLine + 10, lines.length);

  for (let i = fromLine; i < maxSearch; i++) {
    const trimmed = lines[i].trim();

    // Skip blank lines, comment lines, and slash command triggers (e.g. /genswagger)
    if (trimmed === '' || trimmed.startsWith('//') || trimmed.startsWith('/')) {
      continue;
    }

    // Try method with receiver
    const receiverMatch = PATTERNS.funcWithReceiver.exec(trimmed);
    if (receiverMatch) {
      const paramStr = receiverMatch[4];
      return {
        name: receiverMatch[3],
        receiver: receiverMatch[2],
        isEcho: PATTERNS.echoContext.test(paramStr),
        isGin: PATTERNS.ginContext.test(paramStr),
        isStdlib: PATTERNS.stdlibHandler.test(paramStr),
        line: i,
      };
    }

    // Try standalone function
    const standaloneMatch = PATTERNS.funcStandalone.exec(trimmed);
    if (standaloneMatch) {
      const paramStr = standaloneMatch[2];
      return {
        name: standaloneMatch[1],
        receiver: null,
        isEcho: PATTERNS.echoContext.test(paramStr),
        isGin: PATTERNS.ginContext.test(paramStr),
        isStdlib: PATTERNS.stdlibHandler.test(paramStr),
        line: i,
      };
    }

    // If we hit a non-func, non-comment, non-blank line, stop searching
    if (!trimmed.startsWith('func')) {
      break;
    }
  }

  return null;
}

// ═══════════════════════════════════════════════════════════════════════════
// Function Body Extraction
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Extract the function body from the opening { to the matching }.
 * Uses brace counting to handle nested blocks.
 */
function extractFunctionBody(lines: string[], funcLine: number): string {
  let depth = 0;
  let started = false;
  const bodyLines: string[] = [];

  for (let i = funcLine; i < lines.length; i++) {
    const line = lines[i];

    for (const ch of line) {
      if (ch === '{') {
        depth++;
        started = true;
      } else if (ch === '}') {
        depth--;
      }
    }

    if (started) {
      bodyLines.push(line);
    }

    // When depth returns to 0, we've found the closing brace
    if (started && depth === 0) {
      break;
    }
  }

  return bodyLines.join('\n');
}

// ═══════════════════════════════════════════════════════════════════════════
// Body DTO Extraction
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Extract the body DTO type from the function body.
 * Supports multiple binding patterns across frameworks.
 */
function extractBodyDTO(funcBody: string, documentText: string): string | null {
  // 1. Try Echo custom middleware pattern: c.Get("dto").(*dto.LoginRequestDTO)
  const getDtoMatch = PATTERNS.bodyDTOGet.exec(funcBody);
  if (getDtoMatch) {
    return getDtoMatch[1];
  }

  // 2. Try Echo Bind: c.Bind(&req)
  const echoBindMatch = PATTERNS.echoBind.exec(funcBody);
  if (echoBindMatch) {
    return resolveVariableType(echoBindMatch[1], funcBody, documentText);
  }

  // 3. Try Gin ShouldBindJSON/BindJSON
  const ginBindMatch = PATTERNS.ginBind.exec(funcBody);
  if (ginBindMatch) {
    return resolveVariableType(ginBindMatch[1], funcBody, documentText);
  }

  // 4. Try stdlib json.NewDecoder(r.Body).Decode(&req)
  const stdlibMatch = PATTERNS.stdlibJsonDecode.exec(funcBody);
  if (stdlibMatch) {
    return resolveVariableType(stdlibMatch[1], funcBody, documentText);
  }

  return null;
}

/**
 * Try to resolve the type of a variable from its declaration in the function body.
 * Looks for: var req dto.LoginRequest or req := dto.LoginRequest{}
 */
function resolveVariableType(varName: string, funcBody: string, _documentText: string): string | null {
  // Try: var req dto.LoginRequest
  const varRegex = new RegExp(`var\\s+${varName}\\s+([\\w.*]+)`);
  const varMatch = varRegex.exec(funcBody);
  if (varMatch) {
    // Remove leading * if present
    return varMatch[1].replace(/^\*/, '');
  }

  // Try: req := dto.LoginRequest{}
  const shortDeclRegex = new RegExp(`${varName}\\s*:=\\s*([\\w.]+)\\s*\\{`);
  const shortMatch = shortDeclRegex.exec(funcBody);
  if (shortMatch) {
    return shortMatch[1];
  }

  // Try: req := new(dto.LoginRequest)
  const newRegex = new RegExp(`${varName}\\s*:=\\s*new\\s*\\(\\s*([\\w.]+)\\s*\\)`);
  const newMatch = newRegex.exec(funcBody);
  if (newMatch) {
    return newMatch[1];
  }

  return null;
}

// ═══════════════════════════════════════════════════════════════════════════
// Response DTO Extraction
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Extract the response DTO type from c.JSON or response.Wrap calls.
 */
function extractResponseDTO(funcBody: string): string | null {
  // Try response.Wrap pattern first (more specific)
  const wrapMatch = PATTERNS.responseDTOInWrap.exec(funcBody);
  if (wrapMatch) {
    return wrapMatch[1];
  }

  // Fallback to direct c.JSON pattern
  const jsonMatch = PATTERNS.responseDTOInJSON.exec(funcBody);
  if (jsonMatch) {
    return jsonMatch[1];
  }

  // Try Gin-style response
  const ginMatch = PATTERNS.ginResponseDTO.exec(funcBody);
  if (ginMatch) {
    return ginMatch[1];
  }

  return null;
}

// ═══════════════════════════════════════════════════════════════════════════
// Query Parameter Extraction
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Extract query parameters from the function body.
 * Detects c.QueryParam, c.Query, r.URL.Query().Get calls and type-inference wrappers.
 */
function extractQueryParams(funcBody: string): ParamInfo[] {
  const params: ParamInfo[] = [];
  const seen = new Set<string>();

  // Find int-typed query params first
  const intParams = new Set<string>();
  const intRegex = new RegExp(PATTERNS.intQueryParam.source, 'g');
  let intMatch: RegExpExecArray | null;
  while ((intMatch = intRegex.exec(funcBody)) !== null) {
    intParams.add(intMatch[1]);
  }

  // Find bool-typed query params
  const boolParams = new Set<string>();
  const boolRegex = new RegExp(PATTERNS.boolQueryParam.source, 'g');
  let boolMatch: RegExpExecArray | null;
  while ((boolMatch = boolRegex.exec(funcBody)) !== null) {
    boolParams.add(boolMatch[1]);
  }

  // Find float-typed query params
  const floatParams = new Set<string>();
  const floatRegex = new RegExp(PATTERNS.floatQueryParam.source, 'g');
  let floatMatch: RegExpExecArray | null;
  while ((floatMatch = floatRegex.exec(funcBody)) !== null) {
    floatParams.add(floatMatch[1]);
  }

  // Collect from all frameworks
  const allPatterns = [
    PATTERNS.queryParam,
    PATTERNS.ginQueryParam,
    PATTERNS.stdlibQueryParam,
  ];

  for (const pattern of allPatterns) {
    const regex = new RegExp(pattern.source, 'g');
    let match: RegExpExecArray | null;
    while ((match = regex.exec(funcBody)) !== null) {
      const name = match[1];
      if (seen.has(name)) continue;
      seen.add(name);

      let type = 'string';
      if (intParams.has(name)) type = 'int';
      else if (boolParams.has(name)) type = 'boolean';
      else if (floatParams.has(name)) type = 'number';

      params.push({
        name,
        type,
        required: false,
        description: humanizeParamName(name),
      });
    }
  }

  return params;
}

// ═══════════════════════════════════════════════════════════════════════════
// Path Parameter Extraction
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Extract path parameters from the function body and route pattern.
 */
function extractPathParams(
  funcBody: string,
  routePath: string | null
): ParamInfo[] {
  const params: ParamInfo[] = [];
  const seen = new Set<string>();

  // Find int-typed path params
  const intParams = new Set<string>();
  const intRegex = new RegExp(PATTERNS.intPathParam.source, 'g');
  let intMatch: RegExpExecArray | null;
  while ((intMatch = intRegex.exec(funcBody)) !== null) {
    intParams.add(intMatch[1]);
  }

  // Extract from all framework patterns
  const allPatterns = [
    PATTERNS.pathParam,
    PATTERNS.chiPathParam,
    PATTERNS.muxPathParam,
  ];

  for (const pattern of allPatterns) {
    const regex = new RegExp(pattern.source, 'g');
    let match: RegExpExecArray | null;
    while ((match = regex.exec(funcBody)) !== null) {
      const name = match[1];
      if (seen.has(name)) continue;
      seen.add(name);

      params.push({
        name,
        type: intParams.has(name) ? 'int' : 'string',
        required: true,
        description: humanizeParamName(name),
      });
    }
  }

  // Also extract from route pattern (e.g., /users/:id) for params not in body
  if (routePath) {
    const routeRegex = new RegExp(PATTERNS.routePathParam.source, 'g');
    let routeMatch: RegExpExecArray | null;
    while ((routeMatch = routeRegex.exec(routePath)) !== null) {
      const name = routeMatch[1];
      if (seen.has(name)) continue;
      seen.add(name);

      params.push({
        name,
        type: 'string',
        required: true,
        description: humanizeParamName(name),
      });
    }
  }

  return params;
}

// ═══════════════════════════════════════════════════════════════════════════
// Header Parameter Extraction
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Extract header parameters from the function body.
 * Filters out standard headers that aren't API-specific params.
 */
function extractHeaderParams(funcBody: string): ParamInfo[] {
  const params: ParamInfo[] = [];
  const seen = new Set<string>();

  const allPatterns = [
    PATTERNS.echoHeaderGet,
    PATTERNS.ginHeaderGet,
    PATTERNS.stdlibHeaderGet,
  ];

  for (const pattern of allPatterns) {
    const regex = new RegExp(pattern.source, 'g');
    let match: RegExpExecArray | null;
    while ((match = regex.exec(funcBody)) !== null) {
      const name = match[1];
      if (seen.has(name) || IGNORED_HEADERS.has(name)) continue;

      // Skip Authorization header — it's handled by @Security
      if (name.toLowerCase() === 'authorization') continue;

      seen.add(name);

      params.push({
        name,
        type: 'string',
        required: false,
        description: humanizeParamName(name.replace(/^X-/, '')),
      });
    }
  }

  return params;
}

// ═══════════════════════════════════════════════════════════════════════════
// FormData & File Upload Extraction
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Extract formData parameters and file upload params from the function body.
 */
function extractFormDataParams(funcBody: string): {
  formDataParams: ParamInfo[];
  fileParams: ParamInfo[];
  hasFileUpload: boolean;
} {
  const formDataParams: ParamInfo[] = [];
  const fileParams: ParamInfo[] = [];
  const seen = new Set<string>();
  let hasFileUpload = false;

  // FormValue patterns
  const formValuePatterns = [
    PATTERNS.echoFormValue,
    PATTERNS.ginFormValue,
    PATTERNS.stdlibFormValue,
  ];

  for (const pattern of formValuePatterns) {
    const regex = new RegExp(pattern.source, 'g');
    let match: RegExpExecArray | null;
    while ((match = regex.exec(funcBody)) !== null) {
      const name = match[1];
      if (seen.has(name)) continue;
      seen.add(name);

      formDataParams.push({
        name,
        type: 'string',
        required: false,
        description: humanizeParamName(name),
      });
    }
  }

  // File upload patterns
  const filePatterns = [
    PATTERNS.echoFormFile,
    PATTERNS.ginFormFile,
    PATTERNS.stdlibFormFile,
  ];

  for (const pattern of filePatterns) {
    const regex = new RegExp(pattern.source, 'g');
    let match: RegExpExecArray | null;
    while ((match = regex.exec(funcBody)) !== null) {
      const name = match[1];
      if (seen.has(name)) continue;
      seen.add(name);
      hasFileUpload = true;

      fileParams.push({
        name,
        type: 'file',
        required: true,
        description: humanizeParamName(name),
      });
    }
  }

  // Also detect multipart form without specific FormFile call
  if (!hasFileUpload && PATTERNS.stdlibMultipartForm.test(funcBody)) {
    hasFileUpload = true;
  }

  return { formDataParams, fileParams, hasFileUpload };
}

// ═══════════════════════════════════════════════════════════════════════════
// Response Header Extraction
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Extract response headers set in the function body.
 */
function extractResponseHeaders(funcBody: string): ResponseHeaderInfo[] {
  const headers: ResponseHeaderInfo[] = [];
  const seen = new Set<string>();

  const allPatterns = [
    PATTERNS.echoResponseHeader,
    PATTERNS.ginResponseHeader,
    PATTERNS.stdlibResponseHeader,
  ];

  for (const pattern of allPatterns) {
    const regex = new RegExp(pattern.source, 'g');
    let match: RegExpExecArray | null;
    while ((match = regex.exec(funcBody)) !== null) {
      const name = match[1];
      if (seen.has(name) || IGNORED_RESPONSE_HEADERS.has(name)) continue;
      seen.add(name);

      headers.push({
        name,
        type: inferHeaderType(name),
        description: humanizeParamName(name.replace(/^X-/, '')),
      });
    }
  }

  return headers;
}

/**
 * Infer the type of a response header from its name.
 */
function inferHeaderType(name: string): string {
  const lower = name.toLowerCase();
  if (
    lower.includes('count') ||
    lower.includes('total') ||
    lower.includes('limit') ||
    lower.includes('offset') ||
    lower.includes('page')
  ) {
    return 'integer';
  }
  return 'string';
}

// ═══════════════════════════════════════════════════════════════════════════
// Deprecation Detection
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Detect if the function is marked as deprecated.
 * Looks for // Deprecated: or //go:deprecated in comment block above func.
 */
function detectDeprecation(lines: string[], funcLine: number): boolean {
  // Look at up to 15 lines above the function
  for (let i = funcLine - 1; i >= Math.max(0, funcLine - 15); i--) {
    const text = lines[i].trim();

    // Check for deprecated markers
    if (PATTERNS.deprecatedComment.test(text) || PATTERNS.deprecatedDirective.test(text)) {
      return true;
    }

    // Stop at blank lines or non-comment lines (we've passed the comment block)
    if (text === '' || (!text.startsWith('//') && text !== '')) {
      break;
    }
  }

  return false;
}

// ═══════════════════════════════════════════════════════════════════════════
// MIME Type Detection
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Auto-detect Accept and Produce MIME types from the function body.
 */
function detectMimeTypes(
  funcBody: string,
  hasFileUpload: boolean,
  bodyDTOType: string | null
): { acceptMimeTypes: string[]; produceMimeTypes: string[] } {
  const acceptMimeTypes: string[] = [];
  const produceMimeTypes: string[] = [];

  // Accept types
  if (hasFileUpload) {
    acceptMimeTypes.push('multipart/form-data');
  } else if (bodyDTOType) {
    acceptMimeTypes.push('json');
  }

  // Produce types — detect from response methods
  if (PATTERNS.xmlResponse.test(funcBody)) {
    produceMimeTypes.push('xml');
  }
  if (PATTERNS.stringResponse.test(funcBody)) {
    produceMimeTypes.push('plain');
  }
  if (PATTERNS.htmlResponse.test(funcBody)) {
    produceMimeTypes.push('html');
  }

  // Default to json if we have JSON responses and nothing else specific
  if (produceMimeTypes.length === 0) {
    produceMimeTypes.push('json');
  }
  // Also add json if we detected other types but still have c.JSON calls
  if (!produceMimeTypes.includes('json') && /c\.JSON\s*\(/.test(funcBody)) {
    produceMimeTypes.unshift('json');
  }

  return { acceptMimeTypes, produceMimeTypes };
}

// ═══════════════════════════════════════════════════════════════════════════
// Failure Code Detection
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Detect additional failure HTTP status codes used in the function body.
 * Returns unique failure infos that the template engine should emit.
 */
function detectFailureCodes(funcBody: string): FailureInfo[] {
  const failures: FailureInfo[] = [];
  const seen = new Set<number>();

  const checks: Array<{ pattern: RegExp; code: number }> = [
    { pattern: PATTERNS.httpStatusBadRequest, code: 400 },
    { pattern: PATTERNS.httpStatusUnauthorized, code: 401 },
    { pattern: PATTERNS.httpStatusForbidden, code: 403 },
    { pattern: PATTERNS.httpStatusNotFound, code: 404 },
    { pattern: PATTERNS.httpStatusConflict, code: 409 },
    { pattern: PATTERNS.httpStatusUnprocessable, code: 422 },
  ];

  for (const { pattern, code } of checks) {
    if (pattern.test(funcBody) && !seen.has(code)) {
      seen.add(code);
      failures.push({
        code,
        type: '', // Will use default error type from config
        description: STATUS_DESCRIPTIONS[code] ?? `Error ${code}`,
      });
    }
  }

  // Sort by code for consistent output
  failures.sort((a, b) => a.code - b.code);

  return failures;
}

// ═══════════════════════════════════════════════════════════════════════════
// Array Response Detection
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Detect if the response is an array type.
 */
function detectArrayResponse(funcBody: string): boolean {
  return (
    PATTERNS.arrayResponseType.test(funcBody) ||
    PATTERNS.makeSliceResponse.test(funcBody)
  );
}

// ═══════════════════════════════════════════════════════════════════════════
// Tag Inference
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Infer the @Tags value from the file context.
 * Priority: existing @Tags in the file > package path > null.
 */
function inferTag(documentText: string, lines: string[]): string | null {
  // 1. Look for existing @Tags in the file
  const tagMatch = PATTERNS.existingTag.exec(documentText);
  if (tagMatch) {
    return tagMatch[1];
  }

  // 2. Try to infer from the file's package import path
  // Look for Pattern() return value or Group() prefix as a hint
  const patternMatch =
    /func\s+\(\w+\s+\*?\w+\)\s+Pattern\s*\([^)]*\)\s+string\s*\{[^}]*return\s+"\/([^"]*)"/s.exec(
      documentText
    );
  if (patternMatch && patternMatch[1]) {
    return patternMatch[1];
  }

  // 3. Look for sub-group definitions
  const groupMatch = /\.Group\s*\(\s*"\/(\w+)"\s*\)/.exec(documentText);
  if (groupMatch) {
    return groupMatch[1];
  }

  return null;
}

// ═══════════════════════════════════════════════════════════════════════════
// Operation ID Generation
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Generate an operation ID from the function name and receiver.
 * Format: ReceiverType.FunctionName or just FunctionName.
 */
function generateOperationId(funcName: string, receiver: string | null): string {
  if (receiver) {
    return `${receiver}.${funcName}`;
  }
  return funcName;
}

// ═══════════════════════════════════════════════════════════════════════════
// Utility Functions
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Convert a parameter name to a human-readable description.
 * Examples: "id" → "ID", "user_id" → "User ID", "limit" → "Limit"
 */
function humanizeParamName(name: string): string {
  if (name.toLowerCase() === 'id') return 'ID';

  return name
    .replace(/_/g, ' ')
    .replace(/-/g, ' ')
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/\b\w/g, (c) => c.toUpperCase())
    .replace(/\bId\b/g, 'ID');
}
