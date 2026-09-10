# Decisions

Everything here was built, looked at, and removed. The notes exist so that none
of it gets built a second time.

## Tiles

The first version drew services as cards: a bordered, rounded, filled tile with
an icon, a name and a host. It read as a launcher, not as an index, and it made
the page look assembled by a machine rather than written by its owner. Both
section styles are lists now, in the same columns, and a `live` section adds a
status and a tail to each row.

## Monograms

A site with no icon of its own was given a letter in a box. A page of forty
different letters is noisier than a page of forty different favicons, and a
letter claims to mean something. Those rows now draw the same globe a browser
draws, and the eye passes over it.

## Greyscale icons

Considered, to stop the favicons reading as confetti. At this size a logo is
recognised by its colour, not by its shape, so greyscale would have made them
uniform and useless at once. What actually settles the page is the straight
left edge: every row starts its icon in the same 20px column.

## CSS columns

`column-width` gave the newspaper flow for free, and rebalanced whenever the
content changed — so hiding a row while filtering threw whole sections into the
next column, and the page jumped sideways on every keystroke. The sections are
dealt into columns on the server now; a column can only get shorter.

## One list while filtering

Keeping the matches where they sat looked like the honest thing to do: the
page never moves, and a link stays where the eye learned it. It is honest and
it does not work. Spatial memory is worth something while the whole list is
on screen; with three matches left it means scanning the full width of the
page for the one Enter opens, past columns that are now empty. So a query
collapses the columns into a single list under the field, best match first,
with the one Enter opens a line below the cursor. Nothing moves while the
field is empty, which is the state the page is in every time it opens.

## Ranking, rather than the first row that contains the letters

The filter matched a query as a substring of one string per row — name, host
and aliases joined — and the first such row in config order won. It is the
shortest filter that can be written and on this page it is not a search:
every host ends in the same domain, so "lo" answered with the domain of
whatever row came first rather than with the row named Lookout, and one
letter answered with whatever contained that letter. Rows are scored now, in
tiers: the start of a name, the start of an alias, the start of a word inside
either, the initials, the first label of the host — the name a box answers to
on the LAN — and only then a letter found in the middle of a word. A single
character reaches no further than the start of a name, an alias or a word in
one, which is as much as a single character can honestly mean. The rest of a
host is never matched at all: it is shared by forty rows, so matching it is
matching nothing.

The matches are moved into a list of their own rather than left where they sit
and painted in a new order. CSS `order` moves what the eye sees and nothing
else, so tabbing and a screen reader would have gone on reading the page in
the order the columns happened to be packed in.

Subsequence matching, the fzf kind where "ngpm" finds Nginx Proxy Manager,
was not taken. It earns its keep against thousands of paths nobody wrote; a
list of sixty names somebody typed by hand is a list you already know, and
there predictability beats reach — every extra way to match is another way to
answer a two-letter query with a surprise.

## A caption, a placeholder and a line of prose

The field had a label above it, a placeholder inside it, and a sentence below
explaining that Enter opens the first match. Together they said the same thing
on every one of the thousand times the page opens, and they made the product
untranslatable. What is left is a magnifying glass, which says it in no
language, and one word from the config.

## Search suggestions from the engine

Rejected. Completions require sending every keystroke to the search engine,
which is the one thing this page exists not to do.

## Adding links from the page

Rejected as a feature of the page. The config is the source of truth, it lives
in version control, and a machine rewriting YAML loses the comments and the
order its owner keeps in their head. A bookmarklet may one day append to a
separate file the server owns; the config it does not touch.

## A number on every row, all the time

The first version of the tail showed latency on every live row. On a LAN that
is the same three milliseconds every day, and a number that never changes is
furniture — the eye stops reading it, including on the day it changes. The
default says nothing while the last day was perfect and shows the uptime
figure once it was not. Latency is still available (`status.tail: latency`)
for anyone who wants to watch it.

## Proxying icons on demand

The obvious alternative to fetching icons ahead of time: serve `/icon/x` by
going to the site right then, and cache what comes back. It removes a step, and
it costs the two things the step buys. The first paint would wait on however
many sites have no cached icon yet — on a cold cache, all of them. And a start
page that reaches out to forty sites the moment it opens tells each of them
when its owner sat down at the computer, which is the leak the favicon service
was rejected for. Fetching happens when a link is added; a running server does
it in the background for links it has not seen before.

## Resizing the background

The file is served as it is, cached for a day. Resizing it would mean either an
image library or a hand-rolled resampler, for something done once by hand;
`newtab validate` prints a note when the file is over a megabyte instead.

The photograph in the gallery is
[Misty Mountains in Norway](https://commons.wikimedia.org/wiki/File:Misty_Mountains_in_Norway_(Unsplash).jpg),
CC0 — chosen over brighter ones because a background competes with the text
until it stops being interesting.

## Holding the wallpaper in the extension

The live extension briefly kept a copy of the page's background picture and
painted it while the page loaded, so a new tab never showed a flat colour first.
It worked, and it cost a permission prompt over every site you visit plus a
service worker and a daily alarm to notice a replaced wallpaper — all to hide a
fraction of a second. Removed. The extension asks for storage and nothing else,
and the tab waits in a colour you can set by hand.

## A database

There is no state to keep. The page is a function of the config file.
