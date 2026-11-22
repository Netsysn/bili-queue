export function isNearBottom(scrollTop, clientHeight, scrollHeight, threshold = 32) {
  return scrollHeight - (scrollTop + clientHeight) <= threshold;
}
