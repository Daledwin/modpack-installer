package installer

import (
	"modpack-installer/internal/config"
	"modpack-installer/internal/nbt"
)

// mergeServers adds the configured servers to a servers.dat, preserving any
// servers (and their tags) already present. Returns how many were newly added.
func mergeServers(path string, servers []config.Server, dryRun bool) (int, error) {
	root, err := nbt.ReadFile(path)
	if err != nil {
		// Corrupt/unreadable: start fresh rather than fail the whole install.
		root = &nbt.Compound{}
	}

	var lst *nbt.List
	if v, ok := root.Get("servers"); ok {
		lst, _ = v.(*nbt.List)
	}
	if lst == nil {
		lst = &nbt.List{ElemType: nbt.TagCompound}
		root.Set("servers", lst)
	}

	existing := map[string]bool{}
	for _, it := range lst.Items {
		if c, ok := it.(*nbt.Compound); ok {
			if ip, ok := c.Get("ip"); ok {
				if s, ok := ip.(string); ok {
					existing[s] = true
				}
			}
		}
	}

	added := 0
	for _, s := range servers {
		if s.Address == "" || existing[s.Address] {
			continue
		}
		sc := &nbt.Compound{}
		sc.Set("name", s.Name)
		sc.Set("ip", s.Address)
		lst.Items = append(lst.Items, sc)
		existing[s.Address] = true
		added++
	}
	lst.ElemType = nbt.TagCompound

	if !dryRun && added > 0 {
		if err := nbt.WriteFile(path, root); err != nil {
			return added, err
		}
	}
	return added, nil
}
