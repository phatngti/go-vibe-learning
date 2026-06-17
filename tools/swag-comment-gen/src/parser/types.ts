/**
 * Shared type definitions for the Swag Comment Generator extension.
 *
 * These interfaces describe the metadata extracted from Go handler functions
 * and the parameters needed to build Swaggo annotation blocks.
 *
 * Covers ALL Declarative Comments Format from swaggo/swag:
 * https://github.com/swaggo/swag#declarative-comments-format
 */

/** Information about a single parameter (path, query, body, header, or formData). */
export interface ParamInfo {
  /** Parameter name (e.g., "id", "limit", "Authorization") */
  name: string;
  /** Swagger type: "string" | "int" | "integer" | "bool" | "number" | "file" */
  type: string;
  /** Whether the parameter is required */
  required: boolean;
  /** Human-readable description */
  description: string;
  /** Enum constraint values (e.g., ["active", "inactive"]) */
  enums?: string[];
  /** Default value */
  defaultValue?: string;
  /** Minimum string length */
  minLength?: number;
  /** Maximum string length */
  maxLength?: number;
  /** Minimum numeric value */
  minimum?: number;
  /** Maximum numeric value */
  maximum?: number;
  /** Format hint (e.g., "email", "uuid", "date-time", "uri") */
  format?: string;
  /** Collection format for array params: csv, ssv, tsv, pipes, multi */
  collectionFormat?: string;
}

/** Information about a response header. */
export interface ResponseHeaderInfo {
  /** Header name (e.g., "X-Request-Id", "X-Total-Count") */
  name: string;
  /** Swagger type: "string" | "integer" | "number" | "boolean" */
  type: string;
  /** Human-readable description */
  description: string;
}

/** Information about an additional failure response. */
export interface FailureInfo {
  /** HTTP status code (e.g., 403, 404, 409) */
  code: number;
  /** Response type (e.g., "response.ErrorResponse") */
  type: string;
  /** Human-readable description (e.g., "Forbidden") */
  description: string;
}

/** Information about an additional router mapping. */
export interface RouterInfo {
  /** Route path (e.g., "/users/{id}") */
  path: string;
  /** HTTP method (e.g., "get", "post") */
  method: string;
}

/** Metadata extracted from a Go handler function. */
export interface HandlerMetadata {
  /** Function name (e.g., "listUsersHandler") */
  functionName: string;

  /** Receiver type (e.g., "Handler"), null for standalone functions */
  receiverType: string | null;

  /** Whether the function accepts echo.Context as parameter */
  isEchoHandler: boolean;

  /** Whether the function accepts gin.Context as parameter */
  isGinHandler: boolean;

  /** Whether the function uses net/http stdlib pattern */
  isStdlibHandler: boolean;

  /** Whether the function is a recognized HTTP handler of any framework */
  isHttpHandler: boolean;

  /** HTTP method detected from route registration */
  httpMethod: HttpMethod | null;

  /** Full route path detected from route registration (e.g., "/auth/users/:id") */
  routePath: string | null;

  /** Whether auth middleware was detected in the route registration */
  hasAuth: boolean;

  /** Body DTO type if detected (e.g., "dto.LoginRequestDTO") */
  bodyDTOType: string | null;

  /** Response DTO type if detected (e.g., "dto.ListUsersResponse") */
  responseDTOType: string | null;

  /** Path parameters extracted from route pattern and function body */
  pathParams: ParamInfo[];

  /** Query parameters detected in function body */
  queryParams: ParamInfo[];

  /** Header parameters detected in function body (c.Request().Header.Get()) */
  headerParams: ParamInfo[];

  /** Form data parameters detected in function body (c.FormValue()) */
  formDataParams: ParamInfo[];

  /** Whether file upload was detected (c.FormFile()) */
  hasFileUpload: boolean;

  /** File param names detected via c.FormFile() */
  fileParams: ParamInfo[];

  /** Response headers detected (c.Response().Header().Set()) */
  responseHeaders: ResponseHeaderInfo[];

  /** Whether the function is marked as deprecated (// Deprecated: comment) */
  isDeprecated: boolean;

  /** Auto-detected Accept MIME types */
  acceptMimeTypes: string[];

  /** Auto-detected Produce MIME types */
  produceMimeTypes: string[];

  /** Additional failure codes detected from function body */
  additionalFailures: FailureInfo[];

  /** Whether the response is an array type */
  isArrayResponse: boolean;

  /** Whether the response has no body (e.g., 204 No Content) */
  noBodyResponse: boolean;

  /** Additional router mappings (for multiple @Router) */
  additionalRouters: RouterInfo[];

  /** Security scopes for OAuth2 (e.g., ["read", "write"]) */
  securityScopes: string[];

  /** Line number of the function declaration (0-indexed) */
  functionLine: number;

  /** Validation middleware DTO type if detected */
  validationDTOType: string | null;

  /** Inferred tag from package path or existing annotations */
  inferredTag: string | null;

  /** Operation ID — derived from receiver.funcName or funcName */
  operationId: string | null;
}

/** Supported HTTP methods */
export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | 'OPTIONS' | 'HEAD';

/** Which variant of /genswagger the user typed */
export type SlashVariant =
  | 'auto'     // /genswagger       — full auto-detect
  | 'get'      // /genswagger:get   — force GET
  | 'post'     // /genswagger:post  — force POST
  | 'put'      // /genswagger:put   — force PUT
  | 'delete'   // /genswagger:delete — force DELETE
  | 'patch'    // /genswagger:patch — force PATCH
  | 'empty';   // /genswagger:empty — bare placeholders

/** Route mapping extracted from RegisterRoutes() */
export interface RouteMapping {
  /** HTTP method: GET, POST, etc. */
  method: HttpMethod;
  /** Full path including group prefix (e.g., "/auth/users/:id") */
  fullPath: string;
  /** Handler function reference (e.g., "getUserHandler") */
  handlerRef: string;
  /** Whether Authorize() middleware is present */
  hasAuth: boolean;
  /** Whether ValidateHandler middleware is present */
  hasValidation: boolean;
  /** Validation DTO type (e.g., "dto.LoginRequestDTO") */
  validationDTO: string | null;
}
