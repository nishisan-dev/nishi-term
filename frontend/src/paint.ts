// Give React and the WebView a frame to display feedback before synchronous work.
// A bounded fallback also settles when background windows throttle animation frames.
export function afterPaint(): Promise<void> {
  return new Promise((resolve) => {
    let first = 0,
      second = 0;
    const finish = () => {
      clearTimeout(timer);
      cancelAnimationFrame(first);
      cancelAnimationFrame(second);
      resolve();
    };
    const timer = setTimeout(finish, 100);
    first = requestAnimationFrame(() => {
      second = requestAnimationFrame(finish);
    });
  });
}
