import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testTsxSourcemap } from '../utils';

test('script is:inline', async () => {
  const input = `<script is:inline>
  const MyNumber = 3;
  console.log(MyNumber.toStrang());
</script>
`;
  const output = await testTsxSourcemap(input, '\n');

  assert.equal(output, {
    line: 1,
    column: 18,
    source: 'index.astro',
    name: null,
  });
});

test.run();
