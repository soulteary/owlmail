self.addEventListener('notificationclick', (event) => {
    event.notification.close();
    const emailID = event.notification.data && event.notification.data.emailID;
    event.waitUntil((async () => {
        const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
        if (windows.length > 0) {
            const client = windows[0];
            await client.focus();
            client.postMessage({ type: 'owlmail-notification-click', emailID });
            return;
        }
        const client = await self.clients.openWindow('/');
        if (client) client.postMessage({ type: 'owlmail-notification-click', emailID });
    })());
});
