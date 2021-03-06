package softlayer

import (
	"errors"
	"fmt"
	"log"

	"strconv"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/softlayer/softlayer-go/datatypes"
	"github.com/softlayer/softlayer-go/helpers/network"
	"github.com/softlayer/softlayer-go/services"
	"github.com/softlayer/softlayer-go/session"
	"github.com/softlayer/softlayer-go/sl"
)

func resourceSoftLayerLbVpxService() *schema.Resource {
	return &schema.Resource{
		Create:   resourceSoftLayerLbVpxServiceCreate,
		Read:     resourceSoftLayerLbVpxServiceRead,
		Update:   resourceSoftLayerLbVpxServiceUpdate,
		Delete:   resourceSoftLayerLbVpxServiceDelete,
		Exists:   resourceSoftLayerLbVpxServiceExists,
		Importer: &schema.ResourceImporter{},

		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},

			"vip_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"destination_ip_address": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"destination_port": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},

			"weight": {
				Type:     schema.TypeInt,
				Required: true,
			},

			"connection_limit": {
				Type:     schema.TypeInt,
				Required: true,
			},

			"health_check": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func parseServiceId(id string) (string, int, string, error) {
	parts := strings.Split(id, ":")
	vipId := parts[1]
	nacdId, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", -1, "", fmt.Errorf("Error parsing vip id: %s", err)
	}

	serviceName := ""
	if len(parts) > 2 {
		serviceName = parts[2]
	}

	return vipId, nacdId, serviceName, nil
}

func resourceSoftLayerLbVpxServiceCreate(d *schema.ResourceData, meta interface{}) error {
	sess := meta.(*session.Session)
	service := services.GetNetworkApplicationDeliveryControllerService(sess)

	vipId := d.Get("vip_id").(string)
	vipName, nadcId, _, err := parseServiceId(vipId)
	serviceName := d.Get("name").(string)

	if err != nil {
		return fmt.Errorf("Error parsing vip id: %s", err)
	}

	lb_services := []datatypes.Network_LoadBalancer_Service{
		{
			Name:                 sl.String(d.Get("name").(string)),
			DestinationIpAddress: sl.String(d.Get("destination_ip_address").(string)),
			DestinationPort:      sl.Int(d.Get("destination_port").(int)),
			Weight:               sl.Int(d.Get("weight").(int)),
			HealthCheck:          sl.String(d.Get("health_check").(string)),
			ConnectionLimit:      sl.Int(d.Get("connection_limit").(int)),
		},
	}

	lbVip := &datatypes.Network_LoadBalancer_VirtualIpAddress{
		Name:     sl.String(vipName),
		Services: lb_services,
	}

	// Check if there is an existed loadbalancer service which has same name.
	log.Printf("[INFO] Validating LoadBalancer Service Name %s", *lb_services[0].Name)

	_, err = network.GetNadcLbVipServiceByName(sess, nadcId, vipName, serviceName)
	if err == nil {
		return fmt.Errorf("Error creating LoadBalancer Service: The service name '%s' is already used.",
			*lb_services[0].Name)
	}

	log.Printf("[INFO] Creating LoadBalancer Service %s", *lb_services[0].Name)

	successFlag, err := service.Id(nadcId).UpdateLiveLoadBalancer(lbVip)
	if err != nil {
		return fmt.Errorf("Error creating LoadBalancer Service: %s", err)
	}

	if !successFlag {
		return errors.New("Error creating LoadBalancer Service")
	}

	d.SetId(fmt.Sprintf("%s:%s", vipId, serviceName))

	return resourceSoftLayerLbVpxServiceRead(d, meta)
}

func resourceSoftLayerLbVpxServiceRead(d *schema.ResourceData, meta interface{}) error {
	sess := meta.(*session.Session)

	vipName, nadcId, serviceName, err := parseServiceId(d.Id())
	if err != nil {
		return fmt.Errorf("Error parsing vip id: %s", err)
	}

	lbService, err := network.GetNadcLbVipServiceByName(sess, nadcId, vipName, serviceName)
	if err != nil {
		return fmt.Errorf("Unable to get load balancer service: %s", err)
	}

	d.Set("name", *lbService.Name)
	d.Set("destination_ip_address", *lbService.DestinationIpAddress)
	d.Set("destination_port", *lbService.DestinationPort)
	d.Set("weight", *lbService.Weight)
	d.Set("health_check", *lbService.HealthCheck)
	d.Set("connection_limit", *lbService.ConnectionLimit)

	return nil
}

func resourceSoftLayerLbVpxServiceUpdate(d *schema.ResourceData, meta interface{}) error {
	sess := meta.(*session.Session)
	service := services.GetNetworkApplicationDeliveryControllerService(sess)

	vipName, nadcId, _, err := parseServiceId(d.Id())
	if err != nil {
		return fmt.Errorf("Error parsing vip id: %s", err)
	}

	lbService, err := network.GetNadcLbVipServiceByName(sess, nadcId, vipName, d.Get("name").(string))
	if err != nil {
		return fmt.Errorf("Unable to get load balancer service: %s", err)
	}

	// copy current service
	template := datatypes.Network_LoadBalancer_Service(*lbService)

	if data, ok := d.GetOk("name"); ok {
		template.Name = sl.String(data.(string))
	}
	if data, ok := d.GetOk("destination_ip_address"); ok {
		template.DestinationIpAddress = sl.String(data.(string))
	}
	if data, ok := d.GetOk("destination_port"); ok {
		template.DestinationPort = sl.Int(data.(int))
	}
	if data, ok := d.GetOk("weight"); ok {
		template.Weight = sl.Int(data.(int))
	}
	if data, ok := d.GetOk("health_check"); ok {
		template.HealthCheck = sl.String(data.(string))
	}
	if data, ok := d.GetOk("connection_limit"); ok {
		template.ConnectionLimit = sl.Int(data.(int))
	}

	_, err = service.Id(nadcId).UpdateLiveLoadBalancer(&datatypes.Network_LoadBalancer_VirtualIpAddress{
		Name:     sl.String(vipName),
		Services: []datatypes.Network_LoadBalancer_Service{template},
	})
	if err != nil {
		return fmt.Errorf("Error updating LoadBalancer Service: %s", err)
	}

	return nil
}

func resourceSoftLayerLbVpxServiceDelete(d *schema.ResourceData, meta interface{}) error {
	vipName, nadcId, serviceName, err := parseServiceId(d.Id())
	if err != nil {
		return fmt.Errorf("Error parsing vip id: %s", err)
	}

	sess := meta.(*session.Session)
	service := services.GetNetworkApplicationDeliveryControllerService(sess)

	_, err = service.Id(nadcId).DeleteLiveLoadBalancerService(&datatypes.Network_LoadBalancer_Service{
		Name: sl.String(serviceName),
		Vip: &datatypes.Network_LoadBalancer_VirtualIpAddress{
			Name: sl.String(vipName),
		},
	})

	if err != nil {
		return fmt.Errorf("Error deleting Load Balancer Service %s: %s", serviceName, err)
	}

	return nil
}

func resourceSoftLayerLbVpxServiceExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	vipName, nadcId, serviceName, err := parseServiceId(d.Id())
	if err != nil {
		return false, fmt.Errorf("Error parsing vip id: %s", err)
	}

	sess := meta.(*session.Session)
	lbService, err := network.GetNadcLbVipServiceByName(sess, nadcId, vipName, serviceName)
	if err != nil {
		return false, fmt.Errorf("Unable to get load balancer service: %s", err)
	}

	return err == nil && *lbService.Name == serviceName, nil
}
