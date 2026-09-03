export async function withCleanup(operation, cleanup) {
  let primaryError;
  try {
    return await operation();
  } catch (error) {
    primaryError = error;
    throw error;
  } finally {
    try {
      await cleanup();
    } catch (cleanupError) {
      if (primaryError) {
        throw new AggregateError(
          [primaryError, cleanupError],
          "email verification and cleanup both failed",
          { cause: primaryError },
        );
      }
      throw cleanupError;
    }
  }
}
