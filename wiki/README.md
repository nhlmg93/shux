# GitHub Wiki sync

Wiki content in this directory is mirrored to the [shux wiki](https://github.com/nhlmg93/shux/wiki) by [`.github/workflows/publish-wiki.yml`](../.github/workflows/publish-wiki.yml) on push to `master`.

`README.md` is excluded from sync (wiki housekeeping only).

## One-time bootstrap

GitHub does not create the `.wiki.git` backend until the first wiki page exists. Do this once:

1. Open the wiki editor: `gh browse -w` (or visit the wiki tab on GitHub)
2. Create a blank **Home** page and save (content can be replaced on the next sync)
3. Merge `docs/site` to `master`, or run the workflow manually:

```bash
gh workflow run publish-wiki.yml -f strategy=clone
```

To force-replace all wiki content from this folder (destructive):

```bash
gh workflow run publish-wiki.yml -f strategy=init
```

## Manual publish (without the workflow)

After the wiki git repo exists:

```bash
gh auth setup-git
git clone https://github.com/nhlmg93/shux.wiki.git /tmp/shux.wiki
cp wiki/Home.md /tmp/shux.wiki/Home.md
cd /tmp/shux.wiki
git add Home.md
git commit -m "Add wiki home page linking to Starlight docs"
git push
```

## GitHub Pages

Pages deploys the Starlight site from `docs/` via `.github/workflows/docs.yml`. Enable with:

```bash
gh api -X POST repos/nhlmg93/shux/pages -f build_type=workflow
```
