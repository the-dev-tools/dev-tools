#!/usr/bin/env tsx
/**
 * JSON Schema Generator Script
 *
 * This script generates JSON Schema files from Effect Schema definitions.
 * Run with: pnpm tsx src/tools/generate.ts
 *
 * Output:
 * - dist/tools/schemas.json - All tool schemas as JSON
 * - dist/tools/schemas.ts - TypeScript file with const exports
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

import { allToolSchemas, executionSchemas, explorationSchemas, mutationSchemas } from './index.ts';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Output directory
const outDir = path.resolve(__dirname, '../../dist/tools');

// Ensure output directory exists
fs.mkdirSync(outDir, { recursive: true });

// =============================================================================
// Generate JSON file
// =============================================================================

const jsonOutput = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  generatedAt: new Date().toISOString(),
  tools: {
    all: allToolSchemas,
    execution: executionSchemas,
    exploration: explorationSchemas,
    mutation: mutationSchemas,
  },
};

fs.writeFileSync(
  path.join(outDir, 'schemas.json'),
  JSON.stringify(jsonOutput, null, 2),
);

console.log('✓ Generated dist/tools/schemas.json');

// =============================================================================
// Generate TypeScript file (for direct imports without runtime generation)
// =============================================================================

const tsOutput = `/**
 * AUTO-GENERATED FILE - DO NOT EDIT
 * Generated from Effect Schema definitions
 * Run 'pnpm generate:schemas' to regenerate
 */

export const allToolSchemas = ${JSON.stringify(allToolSchemas, null, 2)} as const;

export const executionSchemas = ${JSON.stringify(executionSchemas, null, 2)} as const;

export const explorationSchemas = ${JSON.stringify(explorationSchemas, null, 2)} as const;

export const mutationSchemas = ${JSON.stringify(mutationSchemas, null, 2)} as const;

// Individual schema exports
${allToolSchemas
  .map(
    (schema) =>
      `export const ${schema.name}Schema = ${JSON.stringify(schema, null, 2)} as const;`,
  )
  .join('\n\n')}
`;

fs.writeFileSync(path.join(outDir, 'schemas.ts'), tsOutput);

console.log('✓ Generated dist/tools/schemas.ts');

// =============================================================================
// Summary
// =============================================================================

console.log('\nGenerated schemas summary:');
console.log(`  Execution tools: ${executionSchemas.length}`);
console.log(`  Exploration tools: ${explorationSchemas.length}`);
console.log(`  Mutation tools: ${mutationSchemas.length}`);
console.log(`  Total: ${allToolSchemas.length}`);
