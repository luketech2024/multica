export function workbenchReadingOrder(columns: readonly (readonly string[])[]): string[] {
  const result: string[] = [];
  const height = Math.max(0, ...columns.map((column) => column.length));
  for (let row = 0; row < height; row++) {
    for (const column of columns) {
      const id = column[row];
      if (id) result.push(id);
    }
  }
  return result;
}
