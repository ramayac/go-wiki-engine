package scaffold

// shippedDigests lists the SHA-256 of every version of a retired prompt that
// any release of this repo shipped. A retired file whose bytes match one of
// these digests is pristine, so sync may delete it. Any other bytes mean the
// user edited the file, so sync backs it up first.
//
// The digests cover the canonical file in .wiki-instructions/ and the
// dereferenced copies under .claude/commands/ and .github/prompts/. Those
// copies had the same bytes. Generate the list for one prompt name with this
// command, run at the repo root (replace NAME with onboard, lint, refresh or
// watch):
//
//	for p in .wiki-instructions/NAME.md scaffold/.wiki-instructions/NAME.md \
//	  internal/scaffold/files/.wiki-instructions/NAME.md \
//	  .claude/commands/wiki-NAME.md .github/prompts/wiki-NAME.prompt.md \
//	  scaffold/.claude/commands/wiki-NAME.md scaffold/.github/prompts/wiki-NAME.prompt.md \
//	  internal/scaffold/files/.claude/commands/wiki-NAME.md \
//	  internal/scaffold/files/.github/prompts/wiki-NAME.prompt.md; do
//	  git log --all --format=%H -- "$p" | while read sha; do
//	    [ "$(git ls-tree "$sha" -- "$p" | cut -d' ' -f1)" = 100644 ] || continue
//	    git show "$sha:$p" | shasum -a 256 | cut -d' ' -f1
//	  done
//	done | sort -u
//
// The command skips symlink entries (mode 120000). Sync removes engine-owned
// symlinks by their target instead.
var shippedDigests = map[string][]string{
	"lint": {
		"0bb9a22eb9227eb2970030014ff10f47f0b1fbf9c3e25b78fadfc2803f0b3a98",
		"223ac506820a2d7d0f4e585f98266882efa048005df2774cefa94611edc119bf",
		"54b8982df11e55e648e5e27d9101c0f72981f041ad3a4cd6219d2fa2fb3f08ad",
		"ac89ca6057d9e3c4a2fddab18b55e757df1e92a91b3c8f0dbad6330e1f035c2a",
		"de004c7018f90dfae0aa0dffc08fc4b194f46bdafa8265751591e79e54714700",
	},
	"onboard": {
		"560c9b7accb21956876f407489b0e82588e4e7371e86f6cc81bda7f432de04e4",
		"683f2f9c4b784393f9bdd0f94f8e0217d6c35671ee02c43ff796ee278e00956f",
		"763e557e5050dc1f6bd3009cbff66e73c0477edb57d3ccb6e09c5847971c8ec6",
		"87d023dd61780e66665f6570b046295b8a92f6a4802d505cfbd434963e4b8c54",
		"92bfb130cdd5f7b30b23d9c245c9c8e22fae24cbaa156d9fbcb11025d469a463",
		"ad21f6ad365e62574a18bde416eabfbadd9585d67eeed294dfa13fe637117574",
		"bc88ef39f21420bb53947f473a915d00282185d25a9d476a9707cd4f4f1b4a25",
		"f783df397eed753b969adacde261f840784339b1b834d73db8646d4d3adbe1ba",
	},
	"refresh": {
		"1d0fcc4688c5a48637ac046102a1a38b1ab2532612f8c79a43ab9c2f86b6a662",
		"211f9b1d442231ed7edc4233df8d0d1a894a13dcc9965d77b5001582127ce968",
		"574eb9720ed1b6b953a0870a999ac444ac64f70dc8d9b2e3539fdc70912a25ef",
		"5a978ff5070e81a4b81c4098f56cc9b59d075962ecae6c8a3d1bb1ca12f26605",
		"e3e61a6dda61ba4e6e5fb285a6a674bbcd6da49c139ed0d227bd5174a685b315",
		"f22ff5eee8d026986f94a3483521eb2bb187453244528444dbd427e5a575da9d",
		"faa82adbc0c589152b0d76b0b2d935e3a2787edcde550f3256917964e0f954de",
	},
	"watch": {
		"0ba0119cb716af540d5b14ed15b73f1ef62b772e994f16494cb16665a85cca30",
		"31db030dab4e5a616f7d052fcd72f2e11a57aed77dc625d3adb35622d102f59b",
		"b2f8ffb56fc42b4bc745b21a077180174577cc757dcaf2b16c879e1de5b2021e",
		"b90bc7b5471e93f9263f066e4ff982f3bf699d06d1140cea20b5bb88813cab1a",
		"d2ac26c0958d29dedd05c9a1ee51908dfb9c82c6b31df44eaa7e6c77612a8372",
		"e6858d780cb7e28a0a80aeff2974f70f0defe71defa7ed334021771c599c362f",
		"ea399195191ca0fd13b8a165bbbd2f29c1ac9a0f26bf5cc1ba11dcfde5ea2eb2",
	},
}
