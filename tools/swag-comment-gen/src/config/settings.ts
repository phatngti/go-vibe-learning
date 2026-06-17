/**
 * Extension Configuration — Reads user settings from VS Code workspace/user config.
 *
 * Supports all configuration needed for generating swaggo/swag
 * Declarative Comments Format annotations.
 */

import * as vscode from 'vscode';

/** Extension configuration values */
export interface SwagConfig {
  /** The slash command prefix that triggers generation (e.g., "/genswagger") */
  triggerPrefix: string;
  /** Default @Tags value. Leave empty to auto-detect. */
  defaultTag: string;
  /** Align Swaggo annotation columns with tabs */
  useTabAlignment: boolean;
  /** Default type used for @Failure responses */
  defaultErrorType: string;
  /** Auto-include @Security when auth middleware is detected */
  includeSecurity: boolean;
  /** Security scheme name for @Security annotation */
  securityScheme: string;
  /** Add @Accept for POST/PUT/PATCH handlers */
  includeAccept: boolean;
  /** Include // funcName godoc header line */
  includeGodocHeader: boolean;
  /** Default success HTTP status code for POST handlers */
  postSuccessCode: string;
  /** Auto-generate @ID (operation ID) */
  includeOperationId: boolean;
  /** Default @Produce MIME type */
  defaultProduceType: string;
  /** Default @Accept MIME type */
  defaultAcceptType: string;
  /** Custom @x-{name} extension annotations */
  extensions: Record<string, string>;
  /** Operation ID naming style */
  operationIdStyle: 'camelCase' | 'kebab-case' | 'PascalCase';
}

/**
 * Read the current extension configuration from VS Code settings.
 */
export function getConfig(): SwagConfig {
  const cfg = vscode.workspace.getConfiguration('swagCommentGen');

  return {
    triggerPrefix: cfg.get<string>('triggerPrefix', '/genswagger'),
    defaultTag: cfg.get<string>('defaultTag', ''),
    useTabAlignment: cfg.get<boolean>('useTabAlignment', true),
    defaultErrorType: cfg.get<string>('defaultErrorType', 'response.ErrorResponse'),
    includeSecurity: cfg.get<boolean>('includeSecurity', true),
    securityScheme: cfg.get<string>('securityScheme', 'BearerAuth'),
    includeAccept: cfg.get<boolean>('includeAccept', true),
    includeGodocHeader: cfg.get<boolean>('includeGodocHeader', true),
    postSuccessCode: cfg.get<string>('postSuccessCode', '201'),
    includeOperationId: cfg.get<boolean>('includeOperationId', true),
    defaultProduceType: cfg.get<string>('defaultProduceType', 'json'),
    defaultAcceptType: cfg.get<string>('defaultAcceptType', 'json'),
    extensions: cfg.get<Record<string, string>>('extensions', {}),
    operationIdStyle: cfg.get<'camelCase' | 'kebab-case' | 'PascalCase'>('operationIdStyle', 'camelCase'),
  };
}
