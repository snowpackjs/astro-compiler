import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testTSXSourcemap } from '../utils.js';

const fixture = `---
    const MyVariable = "Astro"

    /** Documentation */
    const MyDocumentedVariable = "Astro"

    /** @author Astro */
    const MyJSDocVariable = "Astro"
---
`;

test('hover I', async () => {
	const input = fixture;
	const output = await testTSXSourcemap(input, 'MyVariable');

	assert.equal(output, {
		line: 2,
		column: 11,
		source: 'index.astro',
		name: null,
	});
});

test('hover II', async () => {
	const input = fixture;
	const output = await testTSXSourcemap(input, 'MyDocumentedVariable');

	assert.equal(output, {
		line: 5,
		column: 11,
		source: 'index.astro',
		name: null,
	});
});

test('hover III', async () => {
	const input = fixture;
	const output = await testTSXSourcemap(input, 'MyJSDocVariable');

	assert.equal(output, {
		line: 8,
		column: 11,
		source: 'index.astro',
		name: null,
	});
});

test.run();
