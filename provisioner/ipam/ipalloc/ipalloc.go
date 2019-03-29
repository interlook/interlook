package ipalloc

import (
	"encoding/json"
	"errors"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"io/ioutil"
	"net"
	"strings"
	"sync"
)

type Extension struct {
	IPStart     string `yaml:"ip_start"`
	IPEnd       string `yaml:"ip_end"`
	NetworkCidr string `yaml:"network_cidr"`
	DbFile      string `yaml:"db_file"`
	shutdown    chan bool
	db          db
	config      *config
}

type config struct {
	IPRange net.Addr
}

type IPAMRecord struct {
	IP   string `json:"ip"`
	Host string `json:"host"`
}

type db struct {
	sync.Mutex
	Records []IPAMRecord `json:"records"`
}

func (d db) isIPFree(ip net.IP) bool {
	for _, rec := range d.Records {
		if ip.String() == rec.IP {
			return false
		}
	}
	return true
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (p *Extension) Start(receive <-chan service.Message, send chan<- service.Message) error {
	// load db from ipalloc
	if err := p.db.load(p.DbFile); err != nil {
		log.Warnf("error loading db ipalloc %v", err.Error())
	}
	p.shutdown = make(chan bool)

	for {
		select {
		case <-p.shutdown:
			log.Debug("Extension ipam.ipalloc shut down")
			return nil

		case msg := <-receive:
			log.Debugf("ipam.ipalloc received message %v\n", msg)

			switch msg.Action {
			case service.DeleteAction:
				msg.Action = service.UpdateAction
				if err := p.deleteService(msg.Service.Name); err != nil {
					log.Errorf("Error deleting service %v", msg.Service.Name, err.Error())
					msg.Error = err.Error()
					send <- msg
					continue
				}
				if err := p.db.save(p.DbFile); err != nil {
					log.Errorf("Error saving flowEntries %v", err)
				}
				msg.Service.PublicIP = ""
				send <- msg
			default:
				// check if service is already defined
				// if yes send back msg with update action
				// if not, get new IPAM, update service def and send back msg
				msg.Action = service.UpdateAction

				if p.serviceExist(&msg) {
					log.Debugf("service %v already exist", msg.Service.Name)
					record := p.db.getServiceByName(msg.Service.Name)
					msg.Service.Name = record.Host
					msg.Service.PublicIP = record.IP

					send <- msg
					continue
				}
				log.Debugf("service %v does not exist, adding", msg.Service.Name)
				ip, err := p.addService(msg.Service.Name)
				if err != nil {
					msg.Error = err.Error()
					send <- msg
					continue
				}
				msg.Service.PublicIP = ip
				send <- msg
			}
		}
	}
}

func (p *Extension) Stop() error {
	p.shutdown <- true
	log.Info("extension ipam.ipalloc down")
	return nil
}

func (p *Extension) deleteService(name string) error {
	for i, v := range p.db.Records {
		if v.Host == name {
			p.db.Records = append(p.db.Records[:i], p.db.Records[i+1:]...)
			return nil
		}
	}
	return errors.New("Could not find record for " + name)
}

func (d db) getServiceByName(name string) (svc IPAMRecord) {
	for k, v := range d.Records {
		if v.Host == name {
			return d.Records[k]
		}
	}
	return svc
}

func (p *Extension) addService(name string) (newIP string, err error) {
	log.Debugf("cidr: %v", p.NetworkCidr)
	ip, ipnet, err := net.ParseCIDR(p.NetworkCidr)
	if err != nil {
		return "", err
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {

		if ip.IsMulticast() || strings.Contains(ipnet.String(), ip.String()) {
			continue
		}
		if p.db.isIPFree(ip) {
			newRec := IPAMRecord{ip.String(), name}
			p.db.Records = append(p.db.Records, newRec)
			if err := p.db.save(p.DbFile); err != nil {
				log.Errorf("Error saving db to %v\n", err)
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("no available IPAM")
}

func (d *db) save(file string) error {
	data, err := json.Marshal(d.Records)
	{
		if err != nil {
			return err
		}
	}
	err = ioutil.WriteFile(file, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (d *db) load(file string) error {
	f, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(f, &d.Records); err != nil {
		return err
	}
	return nil
}

func (p *Extension) serviceExist(msg *service.Message) bool {
	for _, v := range p.db.Records {
		if v.Host == msg.Service.Name {
			return true
		}
	}
	return false
}