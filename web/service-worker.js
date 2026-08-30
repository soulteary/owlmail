self.addEventListener('notificationclick', (event) => {
    event.notification.close();
    const emailID = event.notification.data && event.notification.data.emailID;
    event.waitUntil((async () => {
        const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
        const mailbox = windows.find((client) => {
            try {
                const url = new URL(client.url);
                return url.origin === self.location.origin
                    && (url.pathname === '/' || url.pathname === '/index.html');
            } catch (_) {
                return false;
            }
        });
        if (mailbox) {
            await mailbox.focus();
            mailbox.postMessage({ type: 'owlmail-notification-click', emailID });
            return;
        }
        const target = emailID ? `/?email=${encodeURIComponent(emailID)}` : '/';
        await self.clients.openWindow(target);
    })());
});
