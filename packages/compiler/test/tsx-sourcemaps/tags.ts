import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testTsxSourcemap } from '../utils';

test('tag close', async () => {
  const input = `<Hello></Hello>`;
  const output = await testTsxSourcemap(input, '>');

  assert.equal(output, {
    line: 1,
    column: 6,
    source: 'index.astro',
    name: null,
  });
});

test.run();
