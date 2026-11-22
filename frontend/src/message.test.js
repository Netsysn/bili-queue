import test from 'node:test';
import assert from 'node:assert/strict';
import { messageParts } from './message.js';

test('splits mixed text and repeated emotes, normalizing Bilibili image URLs', () => {
  const parts = messageParts('你好[笑][笑]', {'[笑]': {url:'http://i0.hdslb.com/a.png'}});
  assert.deepEqual(parts, [{text:'你好'}, {text:'[笑]', url:'https://i0.hdslb.com/a.png', large:false}, {text:'[笑]', url:'https://i0.hdslb.com/a.png', large:false}]);
});

test('keeps untrusted HTML and untrusted image addresses as plain text', () => {
  assert.deepEqual(messageParts('<img onerror=alert(1)>[坏]', {'[坏]': {url:'https://hdslb.com.example.org/x'}}), [{text:'<img onerror=alert(1)>'}, {text:'[坏]'}]);
});

test('handles standalone emotes and prefers longer overlapping tokens', () => {
  const parts = messageParts('[大笑]', {'[大笑]':{url:'https://i0.hdslb.com/a.webp'}, '[大':{url:'https://i0.hdslb.com/b.png'}});
  assert.deepEqual(parts, [{text:'[大笑]', url:'https://i0.hdslb.com/a.webp', large:true}]);
});
