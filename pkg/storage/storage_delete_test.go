package storage

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/pyroscope-io/pyroscope/pkg/config"
	"github.com/pyroscope-io/pyroscope/pkg/storage/dict"
	"github.com/pyroscope-io/pyroscope/pkg/storage/dimension"
	"github.com/pyroscope-io/pyroscope/pkg/storage/segment"
	"github.com/pyroscope-io/pyroscope/pkg/storage/tree"
	"github.com/pyroscope-io/pyroscope/pkg/testing"
)

var _ = Describe("storage package", func() {
	var s *Storage

	testing.WithConfig(func(cfg **config.Config) {
		JustBeforeEach(func() {
			var err error
			s, err = New(NewConfig(&(*cfg).Server), logrus.StandardLogger(), prometheus.NewRegistry())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	// Just a sanity check
	Context("basic delete", func() {
		It("deletes trees", func() {
			Expect(s.trees.Cache.Size()).To(Equal(uint64(0)))
			tree := tree.New()
			s.trees.Put("a;b", tree)
			Expect(s.trees.Cache.Size()).To(Equal(uint64(1)))
			s.trees.Delete("a;b")
			Expect(s.trees.Cache.Size()).To(Equal(uint64(0)))
		})

		It("deletes dictionaries", func() {
			Expect(s.dicts.Cache.Size()).To(Equal(uint64(0)))
			d := dict.New()
			s.dicts.Put("dict", d)
			Expect(s.dicts.Cache.Size()).To(Equal(uint64(1)))
			s.dicts.Delete("dict")
			Expect(s.dicts.Cache.Size()).To(Equal(uint64(0)))
		})

		It("deletes segments", func() {
			Expect(s.segments.Cache.Size()).To(Equal(uint64(0)))
			d := segment.New()
			s.segments.Put("segment", d)
			Expect(s.segments.Cache.Size()).To(Equal(uint64(1)))
			s.segments.Delete("segment")
			Expect(s.segments.Cache.Size()).To(Equal(uint64(0)))
		})

		It("deletes dimensions", func() {
			Expect(s.dimensions.Cache.Size()).To(Equal(uint64(0)))
			d := dimension.New()
			s.dimensions.Put("dimensions", d)
			Expect(s.dimensions.Cache.Size()).To(Equal(uint64(1)))
			s.dimensions.Delete("dimensions")
			Expect(s.dimensions.Cache.Size()).To(Equal(uint64(0)))
		})
	})

	Context("delete app", func() {
		/*************************************/
		/*  h e l p e r   f u n c t i o n s  */
		/*************************************/
		checkSegmentsPresence := func(appname string, presence bool) {
			segmentKey, err := segment.ParseKey(string(appname))
			Expect(err).ToNot(HaveOccurred())
			segmentKeyStr := segmentKey.SegmentKey()
			Expect(segmentKeyStr).To(Equal(appname + "{}"))
			_, ok := s.segments.Cache.Lookup(segmentKeyStr)

			if presence {
				Expect(ok).To(BeTrue())
			} else {
				Expect(ok).To(BeFalse())
			}
		}

		checkDimensionsPresence := func(appname string, presence bool) {
			_, ok := s.lookupAppDimension(appname)
			if presence {
				Expect(ok).To(BeTrue())
			} else {
				Expect(ok).To(BeFalse())
			}
		}

		checkTreesPresence := func(appname string, st time.Time, depth int, presence bool) interface{} {
			key, err := segment.ParseKey(appname)
			Expect(err).ToNot(HaveOccurred())
			treeKeyName := key.TreeKey(depth, st)
			t, ok := s.trees.Cache.Lookup(treeKeyName)
			if presence {
				Expect(ok).To(BeTrue())
			} else {
				Expect(ok).To(BeFalse())
			}

			return t
		}

		checkDictsPresence := func(appname string, presence bool) interface{} {
			d, ok := s.dicts.Cache.Lookup(appname)
			if presence {
				Expect(ok).To(BeTrue())
			} else {
				Expect(ok).To(BeFalse())
			}
			return d
		}

		checkLabelsPresence := func(appname string, presence bool) {
			// this indirectly calls s.labels
			appnames := s.GetAppNames()

			// linear scan should be fast enough here
			found := false
			for _, v := range appnames {
				if v == appname {
					found = true
				}
			}

			if presence {
				Expect(found).To(BeTrue())
			} else {
				Expect(found).To(BeFalse())
			}
		}

		Context("simple app", func() {
			It("works correctly", func() {
				appname := "my.app.cpu"

				// We insert info for an app
				tree1 := tree.New()
				tree1.Insert([]byte("a;b"), uint64(1))

				st := testing.SimpleTime(10)
				et := testing.SimpleTime(19)
				key, _ := segment.ParseKey(appname)
				err := s.Put(&PutInput{
					StartTime:  st,
					EndTime:    et,
					Key:        key,
					Val:        tree1,
					SpyName:    "testspy",
					SampleRate: 100,
				})
				Expect(err).ToNot(HaveOccurred())

				// Since the DeleteApp also removes dictionaries
				// therefore we need to create them manually here
				// (they are normally created when TODO)
				d := dict.New()
				s.dicts.Put(appname, d)

				/*******************************/
				/*  S a n i t y   C h e c k s  */
				/*******************************/
				// Dimensions
				Expect(s.dimensions.Cache.Size()).To(Equal(uint64(1)))
				checkDimensionsPresence(appname, true)

				// Trees
				Expect(s.trees.Cache.Size()).To(Equal(uint64(1)))
				t := checkTreesPresence(appname, st, 0, true)
				Expect(t).To(Equal(tree1))

				// Segments
				Expect(s.segments.Cache.Size()).To(Equal(uint64(1)))
				checkSegmentsPresence(appname, true)

				// Dicts
				// I manually inserted a dictionary so it should be fine?
				Expect(s.dicts.Cache.Size()).To(Equal(uint64(1)))
				checkDictsPresence(appname, true)

				// Labels
				checkLabelsPresence(appname, true)

				/*************************/
				/*  D e l e t e   a p p  */
				/*************************/
				err = s.DeleteApp(appname)
				Expect(err).ToNot(HaveOccurred())

				// Trees
				// should've been deleted from CACHE
				Expect(s.trees.Cache.Size()).To(Equal(uint64(0)))
				t = checkTreesPresence(appname, st, 0, false)
				// Trees should've been also deleted from DISK
				// TODO: how to check for that?

				// Dimensions
				Expect(s.dimensions.Cache.Size()).To(Equal(uint64(0)))
				checkDimensionsPresence(appname, false)

				// Dicts
				Expect(s.dicts.Cache.Size()).To(Equal(uint64(0)))
				checkDictsPresence(appname, false)

				// Segments
				Expect(s.segments.Cache.Size()).To(Equal(uint64(0)))
				checkSegmentsPresence(appname, false)

				// Labels
				checkLabelsPresence(appname, false)
			})
		})

		Context("app with labels", func() {
			It("works correctly", func() {
				appname := "my.app.cpu"

				// We insert info for an app
				tree1 := tree.New()
				tree1.Insert([]byte("a;b"), uint64(1))

				// We are mirroring this on the simple.golang.cpu example
				labels := []string{
					"",
					"{foo=bar,function=fast}",
					"{foo=bar,function=slow}",
				}

				st := testing.SimpleTime(10)
				et := testing.SimpleTime(19)
				for _, l := range labels {
					key, _ := segment.ParseKey(appname + l)
					err := s.Put(&PutInput{
						StartTime:  st,
						EndTime:    et,
						Key:        key,
						Val:        tree1,
						SpyName:    "testspy",
						SampleRate: 100,
					})
					Expect(err).ToNot(HaveOccurred())
				}

				// Since the DeleteApp also removes dictionaries
				// therefore we need to create them manually here
				// (they are normally created when TODO)
				d := dict.New()
				s.dicts.Put(appname, d)

				/*******************************/
				/*  S a n i t y   C h e c k s  */
				/*******************************/

				By("checking dimensions were created")
				// Dimensions
				// 4 dimensions
				// i:__name__:my.app.cpu
				// i:foo:bar
				// i:function:fast
				// i:function:slow
				Expect(s.dimensions.Cache.Size()).To(Equal(uint64(4)))
				checkDimensionsPresence(appname, true)

				By("checking trees were created")
				// Trees
				// 3 trees
				// t:my.app.cpu{foo=bar,function=fast}:0:1637853390
				// t:my.app.cpu{foo=bar,function=slow}:0:1637853390
				// t:my.app.cpu{}:0:1637853390
				Expect(s.trees.Cache.Size()).To(Equal(uint64(3)))
				checkTreesPresence(appname, st, 0, true)

				By("checking segments were created")
				// Segments
				// 3 segments
				// s:my.app.app3.cpu{foo=bar,function=fast}
				// s:my.app.app3.cpu{foo=bar,function=slow}
				// s:my.app.app3.cpu{}
				Expect(s.segments.Cache.Size()).To(Equal(uint64(3)))
				checkSegmentsPresence(appname, true)

				By("checking dicts were created")
				// Dicts
				// I manually inserted a dictionary so it should be fine?
				Expect(s.dicts.Cache.Size()).To(Equal(uint64(1)))
				checkDictsPresence(appname, true)

				// Labels
				checkLabelsPresence(appname, true)

				/*************************/
				/*  D e l e t e   a p p  */
				/*************************/
				By("deleting the app")
				err := s.DeleteApp(appname)
				Expect(err).ToNot(HaveOccurred())

				By("checking trees were deleted")
				// Trees
				// should've been deleted from CACHE
				Expect(s.trees.Cache.Size()).To(Equal(uint64(0)))
				checkTreesPresence(appname, st, 0, false)
				// Trees should've been also deleted from DISK
				// TODO: how to check for that?

				// Dimensions
				By("checking dimensions were deleted")
				s.dimensions.Dump()
				Expect(s.dimensions.Cache.Size()).To(Equal(uint64(0)))
				checkDimensionsPresence(appname, false)

				// Dicts
				By("checking dicts were deleted")
				Expect(s.dicts.Cache.Size()).To(Equal(uint64(0)))
				checkDictsPresence(appname, false)

				// Segments
				By("checking segments were deleted")
				Expect(s.segments.Cache.Size()).To(Equal(uint64(0)))
				checkSegmentsPresence(appname, false)

				// Labels
				By("checking labels were deleted")
				checkLabelsPresence(appname, false)
			})
		})

		// In this test we have 2 apps with the same label
		// And deleting one app should not interfer with the labels of the other app
		Context("multiple apps with labels", func() {
			It("works correctly", func() {
			})
		})
	})
})