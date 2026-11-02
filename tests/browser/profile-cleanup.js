const fs = require('node:fs');

async function removeBrowserProfile(profile) {
    // Retry only the observed transient error. A finite outer budget avoids
    // multiplying recursive-removal retries across the profile directory tree.
    for (let attempt = 0; ; attempt++) {
        try {
            await fs.promises.rm(profile, { recursive: true, force: true });
            return;
        } catch (error) {
            if (error.code !== 'ENOTEMPTY' || attempt === 5) {
                throw new Error(`Native Chromium profile cleanup failed; profile retained at ${profile}`, { cause: error });
            }
            await new Promise(resolve => setTimeout(resolve, 100));
        }
    }
}

module.exports = { removeBrowserProfile };
