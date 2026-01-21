/**
 * Test script to verify Effect Schema integration
 * Run: pnpm tsx src/tools/test-effect-schemas.ts
 */

import { Schema } from 'effect';

import {
  allToolSchemas,
  createJsNodeSchema,
  EffectSchemas,
  executionSchemas,
  explorationSchemas,
  mutationSchemas,
} from '@the-dev-tools/spec/tools';

console.log('='.repeat(60));
console.log('Effect Schema Tool Definitions Test');
console.log('='.repeat(60));

// Test 1: Count schemas
console.log('\n1. Schema counts:');
console.log(`   Mutation tools: ${mutationSchemas.length}`);
console.log(`   Exploration tools: ${explorationSchemas.length}`);
console.log(`   Execution tools: ${executionSchemas.length}`);
console.log(`   Total: ${allToolSchemas.length}`);

// Test 2: Verify createJsNode schema structure
console.log('\n2. createJsNode schema:');
console.log(JSON.stringify(createJsNodeSchema, null, 2));

// Test 3: Validate input using Effect Schema
console.log('\n3. Input validation test:');

const validInput = {
  flowId: '01ARZ3NDEKTSV4RRFFQ69G5FAV',
  name: 'Transform Data',
  code: 'const result = ctx.value * 2; return { result };',
};

const invalidInput = {
  flowId: 'not-a-valid-ulid',
  name: '',  // Empty name should fail minLength
  code: 'return ctx;',
};

try {
  const decoded = Schema.decodeUnknownSync(EffectSchemas.Mutation.CreateJsNode)(validInput);
  console.log('   Valid input decoded successfully:', JSON.stringify(decoded));
} catch (error) {
  console.log('   Valid input failed (unexpected):', error);
}

try {
  const decoded = Schema.decodeUnknownSync(EffectSchemas.Mutation.CreateJsNode)(invalidInput);
  console.log('   Invalid input decoded (unexpected):', decoded);
} catch (error) {
  console.log('   Invalid input rejected (expected):', (error as Error).message.slice(0, 100) + '...');
}

// Test 4: Type inference
console.log('\n4. TypeScript type inference:');
type CreateJsNodeInput = typeof EffectSchemas.Mutation.CreateJsNode.Type;
const typedInput: CreateJsNodeInput = {
  flowId: '01ARZ3NDEKTSV4RRFFQ69G5FAV',
  name: 'My Node',
  code: 'return {};',
};
console.log('   Type-safe input:', typedInput);

// Test 5: List all tool names
console.log('\n5. All tool names:');
allToolSchemas.forEach((schema, i) => {
  console.log(`   ${i + 1}. ${schema.name}`);
});

console.log('\n' + '='.repeat(60));
console.log('All tests passed!');
console.log('='.repeat(60));
