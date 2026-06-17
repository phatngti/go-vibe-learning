/**
 * Route Scanner — Finds HTTP method, path, and middleware from route registration.
 *
 * Scans the current Go file for RegisterRoutes() and Pattern() methods
 * to resolve the full route path for a given handler function name.
 *
 * Supports the Echo framework pattern:
 *   g.GET("/path", h.handlerName, h.authenticate.Authorize())
 */

import { HttpMethod, RouteMapping } from './types';

/** Regex patterns for route scanning */
const PATTERNS = {
  /** Match Pattern() method that returns the group prefix */
  pattern: /func\s+\(\w+\s+\*?\w+\)\s+Pattern\s*\([^)]*\)\s+string\s*\{[^}]*return\s+"([^"]*)"/s,

  /** Match route registration: g.METHOD("path", h.handler, ...) */
  routeRegistration:
    /(\w+)\.(GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD)\s*\(\s*"([^"]+)"\s*,\s*(?:\w+\.)?(\w+)/g,

  /** Match group creation: g.Group("/prefix") or variable.Group("/prefix") */
  groupCreation: /(\w+)\s*:?=\s*\w+\.Group\s*\(\s*"([^"]+)"\s*\)/g,

  /** Detect auth middleware in route args */
  authMiddleware: /[Aa]uthenticate\.[Aa]uthorize|[Aa]uthorize\(\)|[Rr]equire[Rr]ole|[Rr]equire[Ss]cope/,

  /** Detect ValidateHandler middleware and extract DTO type */
  validateHandler: /ValidateHandler\s*\(\s*new\s*\(\s*([\w.]+)\s*\)/,
};

/**
 * Scan the document text for all route registrations and return a mapping.
 *
 * @param documentText - Full text of the Go source file.
 * @returns Array of RouteMapping objects.
 */
export function scanRoutes(documentText: string): RouteMapping[] {
  const mappings: RouteMapping[] = [];

  // 1. Find the group prefix from Pattern() method
  const groupPrefix = extractGroupPrefix(documentText);

  // 2. Find all sub-group definitions (e.g., g.Group("/users"))
  const subGroups = extractSubGroups(documentText);

  // 3. Find all route registrations
  const routeRegex = new RegExp(PATTERNS.routeRegistration.source, 'g');
  let match: RegExpExecArray | null;

  while ((match = routeRegex.exec(documentText)) !== null) {
    const [fullMatch, groupVar, method, path, handlerRef] = match;

    // Determine the sub-group prefix for this group variable
    const subPrefix = subGroups.get(groupVar) ?? '';

    // Build the full path
    const fullPath = groupPrefix + subPrefix + path;

    // Check the rest of the line / statement for auth middleware
    const lineEnd = documentText.indexOf('\n', match.index);
    const statementEnd = documentText.indexOf(')', match.index + fullMatch.length);
    const restOfStatement = documentText.substring(
      match.index,
      Math.min(lineEnd > -1 ? lineEnd : documentText.length, statementEnd > -1 ? statementEnd + 1 : documentText.length)
    );

    const hasAuth = PATTERNS.authMiddleware.test(restOfStatement);

    // Check for ValidateHandler
    const validateMatch = PATTERNS.validateHandler.exec(restOfStatement);
    const hasValidation = !!validateMatch;
    const validationDTO = validateMatch ? validateMatch[1] : null;

    mappings.push({
      method: method as HttpMethod,
      fullPath,
      handlerRef,
      hasAuth,
      hasValidation,
      validationDTO,
    });
  }

  return mappings;
}

/**
 * Find the route mapping for a specific handler function name.
 *
 * @param documentText - Full text of the Go source file.
 * @param handlerName - The handler function name to look up.
 * @returns The matching RouteMapping, or null if not found.
 */
export function findRouteForHandler(
  documentText: string,
  handlerName: string
): RouteMapping | null {
  const routes = scanRoutes(documentText);
  return routes.find((r) => r.handlerRef === handlerName) ?? null;
}

/**
 * Extract the group prefix from the Pattern() method.
 * Example: func (h *Handler) Pattern(...) string { return "/auth" }
 */
function extractGroupPrefix(text: string): string {
  const match = PATTERNS.pattern.exec(text);
  return match ? match[1] : '';
}

/**
 * Extract sub-group variable-to-prefix mappings.
 * Example: affiliate := g.Group("/affiliate") → Map { "affiliate" → "/affiliate" }
 */
function extractSubGroups(text: string): Map<string, string> {
  const groups = new Map<string, string>();
  const regex = new RegExp(PATTERNS.groupCreation.source, 'g');
  let match: RegExpExecArray | null;

  while ((match = regex.exec(text)) !== null) {
    groups.set(match[1], match[2]);
  }

  return groups;
}
