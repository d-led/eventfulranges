import { describe, it, expect } from 'vitest';
import { normalize, union, difference } from './intervals.js';

describe('intervals', () => {
  it('normalizes overlapping and adjacent intervals', () => {
    expect(normalize([[0, 2], [2, 4], [3, 5]])).toEqual([[0, 5]]);
    expect(normalize([[5, 8], [0, 3]])).toEqual([[0, 3], [5, 8]]);
  });

  it('drops empty intervals', () => {
    expect(normalize([[2, 2], [1, 3]])).toEqual([[1, 3]]);
  });

  it('subtracts an interval from the middle', () => {
    expect(difference([[0, 10]], [[3, 5]])).toEqual([[0, 3], [5, 10]]);
  });

  it('applies additive wins', () => {
    const adds = union([[0, 4]], [[8, 12]]);
    const removes = [[2, 3], [8, 10]];
    expect(difference(adds, removes)).toEqual([[0, 2], [3, 4], [10, 12]]);
  });

  it('treats touches as separate for half-open intervals', () => {
    expect(difference([[0, 4]], [[2, 2]])).toEqual([[0, 4]]);
    expect(difference([[0, 4]], [[4, 6]])).toEqual([[0, 4]]);
  });
});
