var field = document.getElementById('url');
var state = document.getElementById('state');
var tryIt = document.getElementById('try');
var colour = document.getElementById('colour');
var colourState = document.getElementById('colourstate');

// Local answers immediately and is what a new tab reads; sync is what
// carries the address to your other machines.
chrome.storage.local.get({ url: '', colour: '' }, function (local) {
  colour.value = local.colour;
  if (local.url) {
    show(local.url);
    return;
  }
  chrome.storage.sync.get({ url: '', colour: '' }, function (synced) {
    colour.value = synced.colour;
    show(synced.url);
  });
});

colour.addEventListener('input', function () {
  var value = colour.value.trim();
  if (value !== '' && !/^#[0-9a-f]{6}$/i.test(value)) {
    colourState.textContent = 'six hex digits, like #141312';
    return;
  }
  chrome.storage.local.set({ colour: value });
  chrome.storage.sync.set({ colour: value }, function () {
    colourState.textContent = value === '' ? 'browser default' : 'saved';
  });
});

function show(url) {
  field.value = url;
  tryIt.disabled = url === '';
  state.classList.remove('bad');
  state.textContent = url === '' ? 'not set yet' : 'saved — new tabs open this';
}

field.addEventListener('input', function () {
  var url = field.value.trim();
  if (url !== '' && !/^https?:\/\//i.test(url)) {
    // A new tab that redirects to whatever fits in this box is not
    // something to allow quietly.
    state.classList.add('bad');
    state.textContent = 'needs to start with http:// or https://';
    tryIt.disabled = true;
    return;
  }
  chrome.storage.local.set({ url: url });
  chrome.storage.sync.set({ url: url }, function () { show(url); });
});

tryIt.addEventListener('click', function () {
  // Opening a tab is how you find out whether the address was right, and
  // it is the next thing anyone does anyway.
  chrome.tabs.create({});
  window.close();
});
