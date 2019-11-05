package ipalloc

import (
	"encoding/json"
	"errors"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"io/ioutil"
	"net"
	"strings"
	"sync"
)

type IPAlloc struct {
	IPStart     string `yaml:"ip_start"`
	IPEnd       string `yaml:"ip_end"`
	NetworkCidr string `yaml:"network_cidr"`
	DbFile      string `yaml:"db_file"`
	shutdown    chan bool
	db          db
	config      *config
	wg          sync.WaitGroup
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

func (i *IPAlloc) Start(receive <-chan comm.Message, send chan<- comm.Message) error {
	// load db from ipalloc
	if err := i.db.load(i.DbFile); err != nil {
		log.Warnf("error loading db ipalloc %v", err.Error())
	}
	i.shutdown = make(chan bool)

	for {
		select {
		case <-i.shutdown:
			i.wg.Wait()
			log.Debug("IPAlloc ipam.ipalloc shut down")

			return nil

		case msg := <-receive:
			log.Debugf("ipam.ipalloc received message %v\n", msg)
			i.wg.Add(1)
			switch msg.Action {
			case comm.DeleteAction:
				msg.Action = comm.UpdateAction
				if err := i.deleteService(msg.Service.Name); err != nil {
					log.Errorf("Error deleting service %v %v", msg.Service.Name, err.Error())
					msg.Error = err.Error()
					i.wg.Done()
					send <- msg
					continue
				}
				if err := i.db.save(i.DbFile); err != nil {
					log.Errorf("Error saving flowEntries %v", err)
				}
				msg.Service.PublicIP = ""
				i.wg.Done()
				send <- msg
			default:
				// check if service is already defined
				// if yes send back msg with update action
				// if not, get new IPAM, update service def and send back msg
				msg.Action = comm.UpdateAction

				if i.serviceExist(&msg) {
					log.Debugf("service %v already exist", msg.Service.Name)
					record := i.db.getServiceByName(msg.Service.Name)
					msg.Service.Name = record.Host
					msg.Service.PublicIP = record.IP

					send <- msg
					i.wg.Done()
					continue
				}
				log.Debugf("service %v does not exist, adding", msg.Service.Name)
				ip, err := i.addService(msg.Service.Name)
				if err != nil {
					msg.Error = err.Error()
					send <- msg
					i.wg.Done()
					continue
				}
				msg.Service.PublicIP = ip
				send <- msg
				i.wg.Done()
			}
		}
	}
}

func (i *IPAlloc) Stop() error {
	i.shutdown <- true

	return nil
}

func (i *IPAlloc) deleteService(name string) error {
	for k, v := range i.db.Records {
		if v.Host == name {
			i.db.Records = append(i.db.Records[:k], i.db.Records[k+1:]...)
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

func (i *IPAlloc) addService(name string) (newIP string, err error) {
	log.Debugf("cidr: %v", i.NetworkCidr)
	ip, ipnet, err := net.ParseCIDR(i.NetworkCidr)
	if err != nil {
		return "", err
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {

		if ip.IsMulticast() || strings.Contains(ipnet.String(), ip.String()) {
			continue
		}
		if i.db.isIPFree(ip) {
			newRec := IPAMRecord{ip.String(), name}
			i.db.Records = append(i.db.Records, newRec)
			if err := i.db.save(i.DbFile); err != nil {
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

func (i *IPAlloc) serviceExist(msg *comm.Message) bool {
	for _, v := range i.db.Records {
		if v.Host == msg.Service.Name {
			return true
		}
	}
	return false
}
