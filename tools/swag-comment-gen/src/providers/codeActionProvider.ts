/**
 * Code Action Provider — Shows a lightbulb when cursor is above an undocumented handler.
 *
 * Detects when the cursor is on an empty line or comment line immediately above
 * a Go function declaration that lacks Swaggo annotations, and offers to generate them.
 */

import * as vscode from 'vscode';

export class SwagCodeActionProvider implements vscode.CodeActionProvider {
  public static readonly providedCodeActionKinds = [
    vscode.CodeActionKind.QuickFix,
  ];

  provideCodeActions(
    document: vscode.TextDocument,
    range: vscode.Range | vscode.Selection,
    _context: vscode.CodeActionContext,
    _token: vscode.CancellationToken
  ): vscode.CodeAction[] | undefined {
    // Only activate in Go files
    if (document.languageId !== 'go') {
      return;
    }

    const currentLine = range.start.line;
    const totalLines = document.lineCount;

    // Find the next non-empty, non-comment line below current
    let funcLine = -1;
    for (let i = currentLine + 1; i < Math.min(currentLine + 5, totalLines); i++) {
      const text = document.lineAt(i).text.trim();
      if (text === '' || text.startsWith('//')) continue;
      if (text.startsWith('func ')) {
        funcLine = i;
      }
      break;
    }

    // Also check if current line itself leads into a func
    if (funcLine === -1) {
      const nextLineIdx = currentLine + 1;
      if (nextLineIdx < totalLines) {
        const nextText = document.lineAt(nextLineIdx).text.trim();
        if (nextText.startsWith('func ')) {
          funcLine = nextLineIdx;
        }
      }
    }

    if (funcLine === -1) {
      return;
    }

    // Check if there's already a godoc/swagger comment above the function
    const hasExistingDoc = hasSwaggerDoc(document, funcLine);
    if (hasExistingDoc) {
      return;
    }

    // Check that the function looks like an HTTP handler (any framework)
    const funcText = document.lineAt(funcLine).text;
    const isHandler =
      funcText.includes('echo.Context') ||      // Echo
      funcText.includes('gin.Context') ||        // Gin
      funcText.includes('http.ResponseWriter') || // stdlib / chi / mux
      funcText.includes('http.Request') ||        // stdlib
      funcText.includes('Context');               // generic context param
    if (!isHandler) {
      return;
    }

    // Create the code action
    const generateAction = new vscode.CodeAction(
      '💡 Generate Swaggo Comment',
      vscode.CodeActionKind.QuickFix
    );
    generateAction.command = {
      command: 'swagCommentGen.generate',
      title: 'Generate Swaggo Comment',
    };
    generateAction.isPreferred = true;

    // Also offer the empty template
    const emptyAction = new vscode.CodeAction(
      'Generate Empty Swaggo Template',
      vscode.CodeActionKind.QuickFix
    );
    emptyAction.command = {
      command: 'swagCommentGen.generateEmpty',
      title: 'Generate Empty Swaggo Template',
    };

    return [generateAction, emptyAction];
  }
}

/**
 * Check if the lines above a function already contain Swagger annotations.
 */
function hasSwaggerDoc(document: vscode.TextDocument, funcLine: number): boolean {
  // Look up to 20 lines above for existing swagger comments
  for (let i = funcLine - 1; i >= Math.max(0, funcLine - 20); i--) {
    const text = document.lineAt(i).text.trim();

    // Found swagger annotation
    if (text.includes('@Summary') || text.includes('@Router') || text.includes('godoc')) {
      return true;
    }

    // Hit a blank line that isn't between comment lines → stop
    if (text === '') {
      // Check if the line above is also blank or non-comment
      if (i > 0) {
        const above = document.lineAt(i - 1).text.trim();
        if (!above.startsWith('//')) {
          return false;
        }
      }
    }

    // Hit a non-comment line → no existing doc
    if (!text.startsWith('//') && text !== '') {
      return false;
    }
  }

  return false;
}
