self.addEventListener('notificationclick', (event) => {
    event.notification.close();
    const emailId = event.notification.data && event.notification.data.emailId;

    event.waitUntil((async () => {
        const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
        if (windows.length > 0) {
            const client = windows[0];
            await client.focus();
            if (emailId) client.postMessage({ type: 'owlmail-open-email', emailId });
            return;
        }
        await self.clients.openWindow('/');
    })());
});
