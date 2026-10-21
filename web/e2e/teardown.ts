export default async function teardown() {
  // Let the supervisor stop Arc and remove only its own temporary directory before Playwright
  // terminates the server process (Windows process termination doesn't run signal handlers).
  try {
    const response = await fetch('http://127.0.0.1:18200/shutdown', { method: 'POST' });
    if (!response.ok) throw new Error(await response.text());
  } catch (error) {
    const cause = (error as { cause?: { code?: string } }).cause;
    if (cause?.code !== 'ECONNREFUSED') throw error;
  }
}
