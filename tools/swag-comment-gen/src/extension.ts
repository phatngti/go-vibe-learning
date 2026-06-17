/**
 * Extension Entry Point — Activates the Swag Comment Generator.
 *
 * Registers:
 *   1. CompletionItemProvider with '/' trigger character (for /genswagger)
 *   2. CodeAction provider (lightbulb for undocumented handlers)
 *   3. Command palette commands (generate, generateEmpty, insertFromCompletion)
 */

import * as vscode from 'vscode';
import { SwagCompletionProvider } from './providers/completionProvider';
import { SwagCodeActionProvider } from './providers/codeActionProvider';
import { generateSwaggerComment } from './providers/commandHandler';
import { SlashVariant } from './parser/types';

export function activate(context: vscode.ExtensionContext): void {
  console.log('Swag Comment Generator: activating...');

  // 1. Register CompletionItemProvider with '/' trigger character
  const completionProvider = vscode.languages.registerCompletionItemProvider(
    { scheme: 'file', language: 'go' },
    new SwagCompletionProvider(),
    '/' // Trigger on slash
  );

  // 2. Register CodeAction provider (lightbulb)
  const codeActionProvider = vscode.languages.registerCodeActionsProvider(
    { scheme: 'file', language: 'go' },
    new SwagCodeActionProvider(),
    {
      providedCodeActionKinds: SwagCodeActionProvider.providedCodeActionKinds,
    }
  );

  // 3. Register commands
  const generateCmd = vscode.commands.registerCommand(
    'swagCommentGen.generate',
    () => {
      generateSwaggerComment('auto');
    }
  );

  const generateEmptyCmd = vscode.commands.registerCommand(
    'swagCommentGen.generateEmpty',
    () => {
      generateSwaggerComment('empty');
    }
  );

  const insertFromCompletionCmd = vscode.commands.registerCommand(
    'swagCommentGen.insertFromCompletion',
    (uri: vscode.Uri, position: vscode.Position, variant: string) => {
      generateSwaggerComment(variant as SlashVariant, uri, position);
    }
  );

  // Push all disposables
  context.subscriptions.push(
    completionProvider,
    codeActionProvider,
    generateCmd,
    generateEmptyCmd,
    insertFromCompletionCmd
  );

  console.log('Swag Comment Generator: activated ✅');
}

export function deactivate(): void {
  console.log('Swag Comment Generator: deactivated');
}
