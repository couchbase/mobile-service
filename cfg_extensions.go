package mobile_mds

import "github.com/couchbase/cbgt"


func CfgRemoveAllNodeDefs(cfg cbgt.Cfg) error {

	kinds := []string{cbgt.NODE_DEFS_KNOWN, cbgt.NODE_DEFS_WANTED}
	for _, kind := range kinds {
		nodeDefs, _, err := cbgt.CfgGetNodeDefs(cfg, kind)
		if err != nil {
			return err
		}

		for uuid, _ := range nodeDefs.NodeDefs {

			// special case this UUID
			if uuid == "4aff6d946f40a463" {
				continue
			}

			if err := cbgt.CfgRemoveNodeDef(cfg, kind, uuid, cbgt.VERSION); err != nil {
				return err
			}
		}


	}

	return nil

}