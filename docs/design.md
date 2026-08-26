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

## A database

There is no state to keep. The page is a function of the config file.
