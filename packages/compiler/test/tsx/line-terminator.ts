import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler'; 
    
test('handles non-standard line terminators', async () => {
  const input = ` `;
  let err = 0;
  try {
    await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
  } catch (e) {
    err = 1;
  }
  assert.equal(err, 0, 'did not error');
});

test.run();
