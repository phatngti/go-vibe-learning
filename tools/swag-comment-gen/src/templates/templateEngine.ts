/**
 * Template Engine — Assembles the final Swaggo comment block.
 *
 * Takes HandlerMetadata from the parser and produces a properly formatted
 * Swaggo annotation block. Supports ALL Declarative Comments Format from
 * swaggo/swag: @Summary, @Description, @ID, @Tags, @Accept, @Produce,
 * @Param (path/query/header/formData/body), @Success, @Failure, @Header,
 * @Security, @Router, @Deprecated, @x-{name}.
 *
 * https://github.com/swaggo/swag#api-operation
 */

import { HandlerMetadata, FailureInfo } from '../parser/types';
import { SwagConfig } from '../config/settings';

/**
 * Build a fully populated Swaggo comment block from parsed handler metadata.
 *
 * @param meta - Handler metadata extracted by the Go parser.
 * @param config - Extension configuration.
 * @returns The formatted comment block as a plain string.
 */
export function buildSwaggerComment(
  meta: HandlerMetadata,
  config: SwagConfig
): string {
  const lines: string[] = [];

  // 1. Godoc header
  if (config.includeGodocHeader) {
    lines.push(`// ${meta.functionName} godoc`);
    lines.push('//');
  }

  // 2. @Summary — derive from function name
  const summary = humanizeFunctionName(meta.functionName);
  lines.push(`//\t@Summary\t\t${summary}`);

  // 3. @Description — tab stop placeholder
  lines.push(`//\t@Description\t\${1:${summary}}`);

  // 4. @ID — operation ID
  if (config.includeOperationId) {
    lines.push(`//\t@ID\t\t\t\t${meta.operationId ?? meta.functionName}`);
  }

  // 5. @Tags
  const tag = resolveTag(meta, config);
  lines.push(`//\t@Tags\t\t\t${tag}`);

  // 6. @Accept — content type(s) for request
  const acceptTypes = resolveAcceptTypes(meta, config);
  if (acceptTypes.length > 0) {
    lines.push(`//\t@Accept\t\t\t${acceptTypes.join(',')}`);
  }

  // 7. @Produce — content type(s) for response
  const produceTypes = resolveProduceTypes(meta, config);
  lines.push(`//\t@Produce\t\t${produceTypes.join(',')}`);

  // 8. @Param — path parameters
  for (const p of meta.pathParams) {
    lines.push(formatParam(p, 'path'));
  }

  // 9. @Param — query parameters
  for (const p of meta.queryParams) {
    lines.push(formatParam(p, 'query'));
  }

  // 10. @Param — header parameters
  for (const p of meta.headerParams) {
    lines.push(formatParam(p, 'header'));
  }

  // 11. @Param — formData parameters
  for (const p of meta.formDataParams) {
    lines.push(formatParam(p, 'formData'));
  }

  // 12. @Param — file upload parameters
  for (const p of meta.fileParams) {
    lines.push(formatParam(p, 'formData'));
  }

  // 13. @Param — body
  if (meta.bodyDTOType && !meta.hasFileUpload) {
    lines.push(
      `//\t@Param\t\t\trequest\tbody\t\t${meta.bodyDTOType}\ttrue\t"Request payload"`
    );
  }

  // 14. @Success
  if (meta.noBodyResponse) {
    // 204 No Content — no response type
    lines.push(`//\t@Success\t\t204\t\t"No content"`);
  } else {
    const successCode = resolveSuccessCode(meta, config);
    const respType = meta.responseDTOType ?? '${2:ResponseType}';
    const typeWrapper = meta.isArrayResponse ? '{array}' : '{object}';
    lines.push(
      `//\t@Success\t\t${successCode}\t\t${typeWrapper}\t${respType}\t"\${3:OK}"`
    );
  }

  // 15. @Header — response headers
  for (const h of meta.responseHeaders) {
    // Format: @Header 200 {string} X-Request-Id "description"
    const successCode = meta.noBodyResponse ? '204' : resolveSuccessCode(meta, config);
    lines.push(
      `//\t@Header\t\t\t${successCode}\t\t{${h.type}}\t${h.name}\t"${h.description}"`
    );
  }

  // 16. @Failure — contextual error responses
  const emittedFailureCodes = emitFailureLines(lines, meta, config);

  // 17. @Failure 500 — always include internal error if not already emitted
  if (!emittedFailureCodes.has(500)) {
    lines.push(
      `//\t@Failure\t\t500\t\t{object}\t${config.defaultErrorType}\t"Internal error"`
    );
  }

  // 18. @Security
  if (meta.hasAuth && config.includeSecurity) {
    if (meta.securityScopes.length > 0) {
      lines.push(`//\t@Security\t\t${config.securityScheme}\t[${meta.securityScopes.join(', ')}]`);
    } else {
      lines.push(`//\t@Security\t\t${config.securityScheme}`);
    }
  }

  // 19. @Deprecated
  if (meta.isDeprecated) {
    lines.push('//\t@Deprecated');
  }

  // 20. @x-{name} — extensions from config
  if (config.extensions) {
    for (const [key, value] of Object.entries(config.extensions)) {
      lines.push(`//\t@x-${key}\t\t${value}`);
    }
  }

  // 21. @Router
  const swaggerPath = convertEchoPathToSwagger(meta.routePath);
  const method = (meta.httpMethod ?? '${4:get}').toLowerCase();
  lines.push(`//\t@Router\t\t\t${swaggerPath} [${method}]`);

  // 22. Additional @Router lines (for multiple routers)
  for (const router of meta.additionalRouters) {
    lines.push(`//\t@Router\t\t\t${router.path} [${router.method}]`);
  }

  return lines.join('\n');
}

/**
 * Build the comment block as a VS Code SnippetString body.
 * This allows tab stops to work for interactive editing after insertion.
 */
export function buildSwaggerSnippet(
  meta: HandlerMetadata,
  config: SwagConfig
): string {
  // The buildSwaggerComment already uses ${n:...} tab stop syntax
  // which is compatible with VS Code SnippetString
  return buildSwaggerComment(meta, config);
}

// ═══════════════════════════════════════════════════════════════════════════
// Parameter Formatting
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Format a @Param annotation line with optional constraints.
 *
 * Basic format:   @Param name location type required "description"
 * With enums:     @Param name location type required "description" Enums(a, b, c)
 * With default:   @Param name location type required "description" default(10)
 * With min/max:   @Param name location type required "description" minlength(1) maxlength(100)
 *
 * See: https://github.com/swaggo/swag#param-type
 */
function formatParam(param: import('../parser/types').ParamInfo, location: string): string {
  let line = `//\t@Param\t\t\t${param.name}\t${location}\t\t${param.type}\t${param.required}\t"${param.description}"`;

  // Append optional constraints
  const constraints: string[] = [];

  if (param.enums && param.enums.length > 0) {
    constraints.push(`Enums(${param.enums.join(', ')})`);
  }
  if (param.defaultValue !== undefined) {
    constraints.push(`default(${param.defaultValue})`);
  }
  if (param.minLength !== undefined) {
    constraints.push(`minlength(${param.minLength})`);
  }
  if (param.maxLength !== undefined) {
    constraints.push(`maxlength(${param.maxLength})`);
  }
  if (param.minimum !== undefined) {
    constraints.push(`minimum(${param.minimum})`);
  }
  if (param.maximum !== undefined) {
    constraints.push(`maximum(${param.maximum})`);
  }
  if (param.format) {
    constraints.push(`format(${param.format})`);
  }
  if (param.collectionFormat) {
    constraints.push(`collectionFormat(${param.collectionFormat})`);
  }

  if (constraints.length > 0) {
    line += '\t' + constraints.join('\t');
  }

  return line;
}

// ═══════════════════════════════════════════════════════════════════════════
// Failure Lines Emission
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Emit @Failure lines based on detected status codes and context.
 * Returns the set of emitted codes to avoid duplicates.
 */
function emitFailureLines(
  lines: string[],
  meta: HandlerMetadata,
  config: SwagConfig
): Set<number> {
  const emitted = new Set<number>();

  // Emit from auto-detected failure codes
  for (const failure of meta.additionalFailures) {
    if (emitted.has(failure.code)) continue;
    emitted.add(failure.code);

    const errorType = failure.type || config.defaultErrorType;
    lines.push(
      `//\t@Failure\t\t${failure.code}\t\t{object}\t${errorType}\t"${failure.description}"`
    );
  }

  // Add contextual failures that aren't already detected

  // 400 for handlers with body params
  if (meta.bodyDTOType && !emitted.has(400)) {
    emitted.add(400);
    lines.push(
      `//\t@Failure\t\t400\t\t{object}\t${config.defaultErrorType}\t"Bad request"`
    );
  }

  // 401 for auth-protected handlers
  if (meta.hasAuth && config.includeSecurity && !emitted.has(401)) {
    emitted.add(401);
    lines.push(
      `//\t@Failure\t\t401\t\t{object}\t${config.defaultErrorType}\t"Unauthorized"`
    );
  }

  return emitted;
}

// ═══════════════════════════════════════════════════════════════════════════
// Accept / Produce Resolution
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Resolve Accept MIME types from metadata and config.
 */
function resolveAcceptTypes(meta: HandlerMetadata, config: SwagConfig): string[] {
  // If file upload detected, use multipart/form-data
  if (meta.hasFileUpload || meta.fileParams.length > 0) {
    return ['multipart/form-data'];
  }

  // If formData params but no file, use x-www-form-urlencoded
  if (meta.formDataParams.length > 0) {
    return ['x-www-form-urlencoded'];
  }

  // For methods with body
  const methodsWithBody = ['POST', 'PUT', 'PATCH'];
  if (
    config.includeAccept &&
    meta.httpMethod &&
    methodsWithBody.includes(meta.httpMethod) &&
    meta.bodyDTOType
  ) {
    // Use auto-detected MIME types if available
    if (meta.acceptMimeTypes.length > 0) {
      return meta.acceptMimeTypes;
    }
    return [config.defaultAcceptType ?? 'json'];
  }

  return [];
}

/**
 * Resolve Produce MIME types from metadata and config.
 */
function resolveProduceTypes(meta: HandlerMetadata, config: SwagConfig): string[] {
  if (meta.noBodyResponse) {
    return []; // 204 No Content — no produce needed, but swag still requires it
  }

  if (meta.produceMimeTypes.length > 0) {
    return meta.produceMimeTypes;
  }

  return [config.defaultProduceType ?? 'json'];
}

// ═══════════════════════════════════════════════════════════════════════════
// Helper Functions
// ═══════════════════════════════════════════════════════════════════════════

/**
 * Convert a function name from camelCase/PascalCase to human-readable words.
 * Examples:
 *   "listUsersHandler" → "List users"
 *   "getUserHandler" → "Get user"
 *   "syncUsersHandler" → "Sync users"
 *   "loginUser" → "Login user"
 */
function humanizeFunctionName(name: string): string {
  // Remove common suffixes
  let cleaned = name
    .replace(/Handler$/i, '')
    .replace(/Endpoint$/i, '');

  // Split camelCase into words
  const words = cleaned
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/([A-Z])([A-Z][a-z])/g, '$1 $2')
    .split(/\s+/);

  if (words.length === 0) return name;

  // Capitalize first word, lowercase the rest
  return words
    .map((w, i) => (i === 0 ? w.charAt(0).toUpperCase() + w.slice(1) : w.toLowerCase()))
    .join(' ');
}

/**
 * Resolve the @Tags value from metadata and config.
 */
function resolveTag(meta: HandlerMetadata, config: SwagConfig): string {
  if (config.defaultTag) {
    return config.defaultTag;
  }
  if (meta.inferredTag) {
    return meta.inferredTag;
  }
  return '${5:tag-name}';
}

/**
 * Resolve the success HTTP status code.
 */
function resolveSuccessCode(meta: HandlerMetadata, config: SwagConfig): string {
  if (meta.noBodyResponse) {
    return '204';
  }
  if (meta.httpMethod === 'POST') {
    return config.postSuccessCode;
  }
  if (meta.httpMethod === 'DELETE') {
    return '200';
  }
  return '200';
}

/**
 * Convert Echo route path to Swagger path format.
 * Echo uses `:param`, Swagger uses `{param}`.
 * Example: "/users/:id" → "/users/{id}"
 */
function convertEchoPathToSwagger(routePath: string | null): string {
  if (!routePath) {
    return '${6:/path}';
  }
  return routePath.replace(/:(\w+)/g, '{$1}');
}
