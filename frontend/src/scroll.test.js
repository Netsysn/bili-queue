import test from 'node:test';
import assert from 'node:assert/strict';
import { isNearBottom } from './scroll.js';

test('follows recent messages only when already near the bottom', () => {
  assert.equal(isNearBottom(0, 300, 200), true);
  assert.equal(isNearBottom(680, 300, 1000), true);
  assert.equal(isNearBottom(200, 300, 1000), false);
});
