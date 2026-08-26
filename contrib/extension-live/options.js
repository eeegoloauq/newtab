var field = document.getElementById('url');
var saved = document.getElementById('saved');

chrome.storage.sync.get({ url: '' }, function (cfg) { field.value = cfg.url; });

field.addEventListener('input', function () {
  var url = field.value.trim();
  // Anything but http(s) would be a redirect to somewhere a new tab has
  // no business going.
  if (url !== '' && !/^https?:\/\//i.test(url)) {
    saved.textContent = 'needs to start with http:// or https://';
    return;
  }
  chrome.storage.sync.set({ url: url }, function () {
    saved.textContent = url === '' ? 'cleared' : 'saved';
  });
});
