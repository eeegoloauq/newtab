// Keeps the copies in step with the page. The page's wallpaper address
// carries the file's digest, so a changed wallpaper is a changed
// address: this notices and replaces what is held. Without it the tab
// would keep painting last month's picture until the options page
// happened to be opened.
const DAILY = 'newtab-refresh';

chrome.runtime.onInstalled.addListener(function () {
  chrome.alarms.create(DAILY, { periodInMinutes: 1440 });
  refresh();
});
chrome.runtime.onStartup.addListener(refresh);
chrome.alarms.onAlarm.addListener(function (alarm) {
  if (alarm.name === DAILY) { refresh(); }
});

function refresh() {
  chrome.storage.local.get({ url: '', picture: null }, function (held) {
    if (!held.url) { return; }
    var origin;
    try {
      origin = new URL(held.url).origin + '/*';
    } catch (e) {
      return;
    }
    // Never asks: permission was granted, or this does nothing at all.
    chrome.permissions.contains({ origins: [origin] }, function (allowed) {
      if (!allowed) { return; }
      fetch(new URL('/manifest.webmanifest', held.url).href, { cache: 'no-cache' })
        .then(function (r) { return r.json(); })
        .then(function (doc) {
          var colour = doc.background_color || doc.theme_color || '';
          if (/^#[0-9a-f]{6}$/i.test(colour)) {
            chrome.storage.local.set({ colour: colour });
          }
          if (!doc.background_image) {
            chrome.storage.local.remove('picture');
            return;
          }
          var href = new URL(doc.background_image, held.url).href;
          if (held.picture && held.picture.href === href) { return; }
          return fetch(href)
            .then(function (r) { return r.blob(); })
            .then(function (blob) {
              if (blob.size > 4 * 1024 * 1024) { throw new Error('too big to hold'); }
              return new Promise(function (ok, no) {
                var reader = new FileReader();
                reader.onload = function () { ok(reader.result); };
                reader.onerror = no;
                reader.readAsDataURL(blob);
              });
            })
            .then(function (data) {
              chrome.storage.local.set({ picture: { href: href, data: data } });
            });
        })
        .catch(function () { /* the page is not up; try again tomorrow */ });
    });
  });
}
