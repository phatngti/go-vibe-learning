/**
 * Command Handler — Implements the core generate logic for all trigger methods.
 *
 * Called from:
 *   - Command Palette ("Swag: Generate Swagger Comment")
 *   - CompletionItemProvider (after selecting /genswagger)
 *   - CodeAction lightbulb
 */

import * as vscode from 'vscode';
import { parseGoFunction } from '../parser/goFunctionParser';
import { buildSwaggerSnippet } from '../templates/templateEngine';
import { emptyTemplate } from '../templates/snippets';
import { getConfig } from '../config/settings';
import { SlashVariant } from '../parser/types';

/**
 * Generate and insert a Swagger comment block at the current cursor position.
 *
 * @param variant - Which /genswagger variant to use.
 * @param uri - Optional document URI (for completion-triggered invocations).
 * @param position - Optional cursor position (for completion-triggered invocations).
 */
export async function generateSwaggerComment(
  variant: SlashVariant,
  uri?: vscode.Uri,
  position?: vscode.Position
): Promise<void> {
  // Resolve editor
  let editor: vscode.TextEditor | undefined;
  if (uri) {
    const doc = await vscode.workspace.openTextDocument(uri);
    editor = await vscode.window.showTextDocument(doc);
  } else {
    editor = vscode.window.activeTextEditor;
  }

  if (!editor) {
    vscode.window.showErrorMessage('Swag: No active editor');
    return;
  }

  if (editor.document.languageId !== 'go') {
    vscode.window.showErrorMessage('Swag: This command only works in Go files');
    return;
  }

  const config = getConfig();
  const cursorLine = position?.line ?? editor.selection.active.line;
  const documentText = editor.document.getText();

  // For the empty variant, just insert the template with tab stops
  if (variant === 'empty') {
    const snippet = new vscode.SnippetString(emptyTemplate() + '\n');
    const insertPos = new vscode.Position(cursorLine, 0);
    await editor.insertSnippet(snippet, insertPos);
    return;
  }

  // Parse the Go function below the cursor
  const metadata = parseGoFunction(documentText, cursorLine, variant);

  if (!metadata) {
    vscode.window.showWarningMessage(
      'Swag: No Go handler function found below the cursor. ' +
        'Place cursor on the line above a func declaration.'
    );
    return;
  }

  if (!metadata.isHttpHandler) {
    const proceed = await vscode.window.showWarningMessage(
      'Swag: Function does not appear to be an HTTP handler (no echo.Context, gin.Context, or http.ResponseWriter parameter). Generate anyway?',
      'Yes',
      'No'
    );
    if (proceed !== 'Yes') return;
  }

  // Build the Swagger comment
  const commentBlock = buildSwaggerSnippet(metadata, config);

  // Remove the trigger line (/genswagger) if present before inserting the comment.
  // When triggered from completion, the line may already be empty (insertText = '').
  // When triggered from command palette, the line may still have the trigger text.
  const currentLineText = editor.document.lineAt(cursorLine).text.trim();
  const isTriggerLine =
    currentLineText.startsWith(config.triggerPrefix) ||
    currentLineText.startsWith('/gen') ||
    (position !== undefined && currentLineText === '');

  if (isTriggerLine) {
    const lineRange = editor.document.lineAt(cursorLine).rangeIncludingLineBreak;
    await editor.edit((editBuilder) => {
      editBuilder.delete(lineRange);
    });
  }

  // Insert as a snippet (supports tab stops)
  const snippet = new vscode.SnippetString(commentBlock + '\n');
  const insertPos = new vscode.Position(cursorLine, 0);
  await editor.insertSnippet(snippet, insertPos);
}
