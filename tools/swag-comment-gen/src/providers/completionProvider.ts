/**
 * Completion Provider — Provides /genswagger slash command in the IntelliSense dropdown.
 *
 * Registers '/' as a trigger character. When the user types '/genswagger',
 * offers completion items that replace the text with generated Swagger annotations.
 */

import * as vscode from 'vscode';
import { getConfig } from '../config/settings';

export class SwagCompletionProvider implements vscode.CompletionItemProvider {
  provideCompletionItems(
    document: vscode.TextDocument,
    position: vscode.Position,
    _token: vscode.CancellationToken,
    context: vscode.CompletionContext
  ): vscode.CompletionItem[] | undefined {
    // Only activate in Go files
    if (document.languageId !== 'go') {
      return;
    }

    // Get text on the current line up to cursor
    const lineText = document
      .lineAt(position.line)
      .text.substring(0, position.character);
    const trimmed = lineText.trimStart();

    const config = getConfig();
    const prefix = config.triggerPrefix;

    // Check if user is typing one of our slash commands
    // Must match the beginning of the trigger prefix
    if (!trimmed.startsWith('/') && context.triggerCharacter !== '/') {
      return;
    }

    // Don't activate inside Go comments that look like swagger annotations
    if (trimmed.startsWith('//\t@') || trimmed.startsWith('// @')) {
      return;
    }

    const variants = [
      {
        label: prefix,
        detail: 'Auto-detect handler and generate full Swaggo annotation',
        variant: 'auto',
      },
      {
        label: `${prefix}:get`,
        detail: 'Force GET handler template',
        variant: 'get',
      },
      {
        label: `${prefix}:post`,
        detail: 'Force POST handler template with body @Param',
        variant: 'post',
      },
      {
        label: `${prefix}:put`,
        detail: 'Force PUT handler template with body @Param',
        variant: 'put',
      },
      {
        label: `${prefix}:delete`,
        detail: 'Force DELETE handler template',
        variant: 'delete',
      },
      {
        label: `${prefix}:patch`,
        detail: 'Force PATCH handler template with body @Param',
        variant: 'patch',
      },
      {
        label: `${prefix}:empty`,
        detail: 'Bare-bones template with all placeholders',
        variant: 'empty',
      },
    ];

    return variants.map((v) => {
      const item = new vscode.CompletionItem(
        v.label,
        vscode.CompletionItemKind.Snippet
      );
      item.detail = v.detail;
      item.documentation = new vscode.MarkdownString(
        `Generate a Swaggo comment block for the handler function below.\n\n` +
          `Variant: **${v.variant}**`
      );
      item.sortText = '!0'; // Sort to the very top
      item.filterText = trimmed;
      item.preselect = v.variant === 'auto';

      // Don't insert the label text — the command handler will manage everything
      item.insertText = '';

      // Command that will execute when the item is selected
      item.command = {
        command: 'swagCommentGen.insertFromCompletion',
        title: 'Insert Swagger Comment',
        arguments: [document.uri, position, v.variant],
      };

      return item;
    });
  }
}
