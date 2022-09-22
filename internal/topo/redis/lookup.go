// Copyright 2022 EMQ Technologies Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build redisdb || !core

package redis

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/lf-edge/ekuiper/pkg/api"
	"github.com/lf-edge/ekuiper/pkg/cast"
)

type conf struct {
	// host:port address.
	Addr     string `json:"addr,omitempty"`
	Username string `json:"username,omitempty"`
	// Optional password. Must match the password specified in the
	Password string `json:"password,omitempty"`
	DataType string `json:"dataType,omitempty"`
}

type lookupSource struct {
	c   *conf
	db  int
	cli *redis.Client
}

func (s *lookupSource) Configure(datasource string, props map[string]interface{}) error {
	db, err := cast.ToInt(datasource, cast.CONVERT_ALL)
	if err != nil {
		return fmt.Errorf("invalid datasource, it must be an integer but got %s", datasource)
	}
	s.db = db
	cfg := &conf{}
	err = cast.MapToStruct(props, cfg)
	if err != nil {
		return err
	}
	if cfg.Addr == "" {
		return errors.New("redis addr is null")
	}
	if cfg.DataType != "string" && cfg.DataType != "list" {
		return errors.New("redis dataType must be string or list")
	}
	s.c = cfg
	return nil
}

func (s *lookupSource) Open(ctx api.StreamContext) error {
	ctx.GetLogger().Infof("Opening redis lookup source with conf %v", s.c)
	s.cli = redis.NewClient(&redis.Options{
		Addr:     s.c.Addr,
		Username: s.c.Username,
		Password: s.c.Password,
		DB:       s.db,
	})
	return nil
}

func (s *lookupSource) Lookup(ctx api.StreamContext, keys []string, values []interface{}) ([]api.SourceTuple, error) {
	ctx.GetLogger().Debugf("Lookup redis %v", keys)
	if len(keys) != 1 {
		return nil, fmt.Errorf("redis lookup only support one key, but got %v", keys)
	}
	v := fmt.Sprintf("%v", values[0])
	if s.c.DataType == "string" {
		res, err := s.cli.Get(v).Result()
		if err != nil {
			if err == redis.Nil {
				return []api.SourceTuple{}, nil
			}
			return nil, err
		}
		m := make(map[string]interface{})
		err = json.Unmarshal([]byte(res), &m)
		if err != nil {
			return nil, err
		}
		return []api.SourceTuple{api.NewDefaultSourceTuple(m, nil)}, nil
	} else {
		res, err := s.cli.LRange(v, 0, -1).Result()
		if err != nil {
			if err == redis.Nil {
				return []api.SourceTuple{}, nil
			}
			return nil, err
		}
		ret := make([]api.SourceTuple, 0, len(res))
		for _, r := range res {
			m := make(map[string]interface{})
			err = json.Unmarshal([]byte(r), &m)
			if err != nil {
				return nil, err
			}
			ret = append(ret, api.NewDefaultSourceTuple(m, nil))
		}
		return ret, nil
	}
}

func (s *lookupSource) Close(ctx api.StreamContext) error {
	ctx.GetLogger().Infof("Closing redis lookup source")
	return s.cli.Close()
}

func GetLookupSource() api.LookupSource {
	return &lookupSource{}
}
