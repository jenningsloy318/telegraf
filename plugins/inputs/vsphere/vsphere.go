package vsphere

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

type VSphere struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

var sampleConfig = `
  ## FQDN or an IP of a vCenter Server or ESXi host
  server = "vcenter.domain.com"

  ## A vSphere/ESX user
  ## must have System.View and Performance.ModifyIntervals privileges
  username = "root"

  ## Password
  password = "vmware"

  ## Do not validate server's TLS certificate
  # insecure =  true
`

func (v *VSphere) Description() string {
	return "Collect metrics from VMware vSphere"
}

func (v *VSphere) SampleConfig() string {
	return sampleConfig
}

func (v *VSphere) gatherDatastoreMetrics(acc telegraf.Accumulator, ctx context.Context, c *govmomi.Client, pc *property.Collector, dss []*object.Datastore) {
	// Convert datastores into list of references
	var refs []types.ManagedObjectReference
	for _, ds := range dss {
		refs = append(refs, ds.Reference())
	}

	// Retrieve summary property for all datastores
	var dst []mo.Datastore
	err := pc.Retrieve(ctx, refs, []string{"summary"}, &dst)
	if err != nil {
		panic(err)
	}

	for _, ds := range dst {

		records := make(map[string]interface{})
		tags := make(map[string]string)

		tags["name"] = ds.Summary.Name
		tags["type"] = ds.Summary.Type
		tags["url"] = ds.Summary.Url

		records["capacity"] = ds.Summary.Capacity
		records["freespace"] = ds.Summary.FreeSpace

		acc.AddFields("ds_metrics", records, tags)
	}
}

func (v *VSphere) gatherVMMetrics(acc telegraf.Accumulator, ctx context.Context, c *govmomi.Client, pc *property.Collector, vms []*object.VirtualMachine) {
	// Convert datastores into list of references
	var refs []types.ManagedObjectReference
	for _, vm := range vms {
		refs = append(refs, vm.Reference())
	}

	// Retrieve name property for all vms
	var vmt []mo.VirtualMachine
	err := pc.Retrieve(ctx, refs, []string{"name", "config", "summary"}, &vmt)
	if err != nil {
		panic(err)
	}

	for _, vm := range vmt {

		records := make(map[string]interface{})
		tags := make(map[string]string)

		tags["name"] = vm.Name
		tags["guest_full_name"] = vm.Config.GuestFullName
		tags["connection_state"] = string(vm.Summary.Runtime.ConnectionState)
		tags["overall_status"] = string(vm.Summary.OverallStatus)
		tags["vm_path_name"] = vm.Summary.Config.VmPathName
		tags["ip_address"] = vm.Summary.Guest.IpAddress
		tags["hostname"] = vm.Summary.Guest.HostName
		tags["guest_id"] = vm.Config.GuestId
		tags["is_guest_tools_running"] = vm.Summary.Guest.ToolsRunningStatus

		records["mem_mb"] = vm.Config.Hardware.MemoryMB
		records["num_cpu"] = vm.Config.Hardware.NumCPU
		records["host_mem_usage"] = vm.Summary.QuickStats.HostMemoryUsage
		records["guest_mem_usage"] = vm.Summary.QuickStats.GuestMemoryUsage
		records["overall_cpu_usage"] = vm.Summary.QuickStats.OverallCpuUsage
		records["overall_cpu_demand"] = vm.Summary.QuickStats.OverallCpuDemand
		records["swap_mem"] = vm.Summary.QuickStats.SwappedMemory
		records["uptime_sec"] = vm.Summary.QuickStats.UptimeSeconds
		records["storage_committed"] = vm.Summary.Storage.Committed
		records["storage_uncommitted"] = vm.Summary.Storage.Uncommitted
		records["max_cpu_usage"] = vm.Summary.Runtime.MaxCpuUsage
		records["max_mem_usage"] = vm.Summary.Runtime.MaxMemoryUsage
		records["num_cores_per_socket"] = vm.Config.Hardware.NumCoresPerSocket

		acc.AddFields("vm_metrics", records, tags)
	}
}

func (v *VSphere) Gather(acc telegraf.Accumulator) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse URL from string
	u, err := url.Parse(fmt.Sprintf("https://%s:%s@%s/sdk", v.Username, v.Password, v.Server))
	if err != nil {
		return err
	}

	// Connect and log in to ESX or vCenter
	c, err := govmomi.NewClient(ctx, u, v.Insecure)
	if err != nil {
		return err
	}
	f := find.NewFinder(c.Client, true)

	// Find one and only datacenter
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		return err
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	pc := property.DefaultCollector(c.Client)

	dss, err := f.DatastoreList(ctx, "*")
	if err != nil {
		return err
	}

	v.gatherDatastoreMetrics(acc, ctx, c, pc, dss)

	// Find virtual machines in datacenter
	vms, err := f.VirtualMachineList(ctx, "*")
	if err != nil {
		return err
	}
	v.gatherVMMetrics(acc, ctx, c, pc, vms)

	return nil
}

func init() {
	inputs.Add("vsphere", func() telegraf.Input { return &VSphere{} })
}
