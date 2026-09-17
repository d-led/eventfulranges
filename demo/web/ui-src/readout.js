// The chain of counts the view reports: the operations the session was given,
// the partition cells they became, and the ranges on screen now. It reads left
// to right in the order the counts are produced, and it is the whole story —
// the same two operations read as one range under merge adjacent and as five
// partition cells under partition.
//
// A count is left out when it adds nothing. Without a partition there are no
// cells, and in partition mode the cells ARE the result (nothing is joined back
// up), so there the two counts would print the same number twice.

export function readout({ ops = 0, cells = 0, ranges = 0 }) {
  const steps = [];
  if (ops > 0) steps.push(count(ops, 'op'));
  if (cells > ranges) steps.push(count(cells, 'partition cell'));
  steps.push(count(ranges, 'range'));
  return steps.join(' → ');
}

function count(n, noun) {
  return `${n} ${noun}${n === 1 ? '' : 's'}`;
}
